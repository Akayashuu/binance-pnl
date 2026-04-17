// Package queries holds read-side use cases.
package queries

import (
	"context"
	"fmt"
	"time"

	"github.com/binancetracker/binancetracker/internal/application/ports"
	"github.com/binancetracker/binancetracker/internal/domain/acquisition"
	"github.com/binancetracker/binancetracker/internal/domain/pnl"
	"github.com/binancetracker/binancetracker/internal/domain/position"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/binancetracker/binancetracker/internal/domain/trade"
	"github.com/shopspring/decimal"
)

type PortfolioOverview struct {
	Quote         shared.Symbol
	Positions     []pnl.Result
	TotalInvested shared.Money
	TotalValue    shared.Money
	UnrealizedPnL shared.Money
	RealizedPnL   shared.Money
	TotalPnL      shared.Money
	GeneratedAt   time.Time
}

type GetPortfolio struct {
	trades         ports.TradeRepository
	acquisitions   ports.AcquisitionRepository
	prices         ports.PriceRepository
	fx             ports.FxRateProvider
	calc           pnl.Calculator
	quote          shared.Symbol
	acceptedQuotes []shared.Symbol
	now            func() time.Time
}

func NewGetPortfolio(
	trades ports.TradeRepository,
	acquisitions ports.AcquisitionRepository,
	prices ports.PriceRepository,
	fx ports.FxRateProvider,
	quote shared.Symbol,
	acceptedQuotes []shared.Symbol,
) *GetPortfolio {
	return &GetPortfolio{
		trades:         trades,
		acquisitions:   acquisitions,
		prices:         prices,
		fx:             fx,
		calc:           pnl.NewCalculator(),
		quote:          quote,
		acceptedQuotes: acceptedQuotes,
		now:            time.Now,
	}
}

func (uc *GetPortfolio) Execute(ctx context.Context) (PortfolioOverview, error) {
	allTrades, err := uc.trades.ListAll(ctx)
	if err != nil {
		return PortfolioOverview{}, fmt.Errorf("list trades: %w", err)
	}
	allAcqs, err := uc.acquisitions.ListAll(ctx)
	if err != nil {
		return PortfolioOverview{}, fmt.Errorf("list acquisitions: %w", err)
	}

	// Bring everything into the configured quote so the AVCO aggregate sees
	// fiat-buy trades and treats acquisitions as synthetic BUYs. Both lists
	// run through the same normalize step so an EUR manual fund and an EUR
	// fiat-buy trade are converted identically.
	merged := append([]trade.Trade(nil), allTrades...)
	merged = append(merged, acquisitionsAsBuyTrades(allAcqs)...)
	normalized := normalizeTradesToQuote(ctx, uc.fx, merged, uc.quote)

	// Accepted quote currencies (USDT, USDC, EUR…) are aggregated as cash
	// positions further down — skip them here to avoid emitting two rows for
	// the same asset when the user has both bought it as a base (e.g. fiat-buy
	// EUR→USDT) and held it as a quote balance.
	acceptedQuoteSet := make(map[shared.Symbol]struct{}, len(uc.acceptedQuotes))
	for _, q := range uc.acceptedQuotes {
		acceptedQuoteSet[q] = struct{}{}
	}

	byAsset := make(map[shared.Symbol][]trade.Trade)
	for _, t := range normalized {
		byAsset[t.Asset()] = append(byAsset[t.Asset()], t)
	}

	overview := PortfolioOverview{
		Quote:         uc.quote,
		TotalInvested: shared.ZeroMoney(uc.quote),
		TotalValue:    shared.ZeroMoney(uc.quote),
		UnrealizedPnL: shared.ZeroMoney(uc.quote),
		RealizedPnL:   shared.ZeroMoney(uc.quote),
		TotalPnL:      shared.ZeroMoney(uc.quote),
		GeneratedAt:   uc.now(),
	}

	for sym, ts := range byAsset {
		if _, isCash := acceptedQuoteSet[sym]; isCash {
			continue
		}
		pos, err := position.Build(sym, uc.quote, ts)
		if err != nil {
			return PortfolioOverview{}, fmt.Errorf("build position %s: %w", sym, err)
		}

		if pos.HeldQuantity().IsZero() && pos.RealizedPnL().IsZero() {
			continue
		}

		price, _, err := uc.prices.Latest(ctx, sym)
		if err != nil {
			price = pos.AverageCost()
		}

		result, err := uc.calc.Calculate(pos, price)
		if err != nil {
			return PortfolioOverview{}, fmt.Errorf("calculate pnl %s: %w", sym, err)
		}
		overview.Positions = append(overview.Positions, result)

		if overview.TotalInvested, err = overview.TotalInvested.Add(pos.TotalInvested()); err != nil {
			return PortfolioOverview{}, err
		}
		if overview.TotalValue, err = overview.TotalValue.Add(result.MarketValue); err != nil {
			return PortfolioOverview{}, err
		}
		if overview.UnrealizedPnL, err = overview.UnrealizedPnL.Add(result.UnrealizedPnL); err != nil {
			return PortfolioOverview{}, err
		}
		if overview.RealizedPnL, err = overview.RealizedPnL.Add(result.RealizedPnL); err != nil {
			return PortfolioOverview{}, err
		}
		if overview.TotalPnL, err = overview.TotalPnL.Add(result.TotalPnL); err != nil {
			return PortfolioOverview{}, err
		}
	}

	// Cash balances for accepted quote currencies (USDT, USDC…).
	cashPositions := uc.buildCashPositions(ctx, allTrades, allAcqs)
	for _, cp := range cashPositions {
		overview.Positions = append(overview.Positions, cp)
		if overview.TotalValue, err = overview.TotalValue.Add(cp.MarketValue); err != nil {
			return PortfolioOverview{}, err
		}
	}

	return overview, nil
}

// buildCashPositions computes the net balance of each accepted quote currency
// from raw (un-normalized) trade flows and deposits, then returns a synthetic
// pnl.Result per positive balance so stablecoins appear in the positions table.
func (uc *GetPortfolio) buildCashPositions(
	ctx context.Context,
	trades []trade.Trade,
	acqs []acquisition.Acquisition,
) []pnl.Result {
	// Identify accepted quote currencies.
	accepted := make(map[string]shared.Symbol, len(uc.acceptedQuotes))
	for _, q := range uc.acceptedQuotes {
		accepted[q.String()] = q
	}

	// Compute net cash flow per accepted quote.
	balances := make(map[string]decimal.Decimal, len(accepted))
	for _, t := range trades {
		// (1) Accepted quote acting as the QUOTE side of a trade — buying
		// drains the balance, selling adds to it.
		qStr := t.Quote().String()
		if _, ok := accepted[qStr]; ok {
			gross := t.Quantity().Decimal().Mul(t.Price().Amount())
			switch t.Side() {
			case trade.SideBuy:
				cost := gross
				if t.Fee().Currency().Equals(t.Quote()) {
					cost = cost.Add(t.Fee().Amount())
				}
				balances[qStr] = balances[qStr].Sub(cost)
			case trade.SideSell:
				proceeds := gross
				if t.Fee().Currency().Equals(t.Quote()) {
					proceeds = proceeds.Sub(t.Fee().Amount())
				}
				balances[qStr] = balances[qStr].Add(proceeds)
			}
		}
		// (2) Accepted quote acting as the ASSET side — e.g. fiat-buy
		// EUR→USDT credits the USDT balance; selling USDT for something
		// else debits it. Without this branch we'd lose those flows.
		aStr := t.Asset().String()
		if _, ok := accepted[aStr]; ok {
			qty := t.Quantity().Decimal()
			switch t.Side() {
			case trade.SideBuy:
				balances[aStr] = balances[aStr].Add(qty)
			case trade.SideSell:
				balances[aStr] = balances[aStr].Sub(qty)
			}
		}
	}
	// Add deposits of accepted quote currencies themselves.
	for _, a := range acqs {
		aStr := a.Asset().String()
		if _, ok := accepted[aStr]; !ok {
			continue
		}
		balances[aStr] = balances[aStr].Add(a.Quantity().Decimal())
	}

	var results []pnl.Result
	for qStr, bal := range balances {
		if !bal.IsPositive() {
			continue
		}
		sym := accepted[qStr]

		qty, err := shared.NewQuantity(bal)
		if err != nil {
			continue
		}

		// Price of this stablecoin in the primary quote.
		var price shared.Money
		if sym.Equals(uc.quote) {
			price, _ = shared.NewMoney(decimal.NewFromInt(1), uc.quote)
		} else {
			if rate, err := uc.fx.Rate(ctx, sym, uc.quote); err == nil {
				price, _ = shared.NewMoney(rate, uc.quote)
			} else {
				price, _ = shared.NewMoney(decimal.NewFromInt(1), uc.quote)
			}
		}

		marketValue := price.MulQuantity(qty)
		costBasis := price.MulQuantity(qty)
		zero := shared.ZeroMoney(uc.quote)

		results = append(results, pnl.Result{
			Asset:         sym,
			Quote:         uc.quote,
			HeldQuantity:  qty,
			AverageCost:   price,
			CurrentPrice:  price,
			MarketValue:   marketValue,
			CostBasis:     costBasis,
			UnrealizedPnL: zero,
			RealizedPnL:   zero,
			TotalPnL:      zero,
		})
	}
	return results
}

// acquisitionsAsBuyTrades wraps each Acquisition as a synthetic BUY trade so
// the position machinery can consume it without a parallel code path. The
// acquisition keeps its native quote currency — callers run the result
// through normalizeTradesToQuote afterwards if they need a uniform quote.
func acquisitionsAsBuyTrades(items []acquisition.Acquisition) []trade.Trade {
	out := make([]trade.Trade, 0, len(items))
	for _, a := range items {
		zeroFee, err := shared.NewMoney(decimal.Zero, a.Quote())
		if err != nil {
			continue
		}
		tr, err := trade.New(trade.Params{
			ID:         a.ID(),
			Asset:      a.Asset(),
			Quote:      a.Quote(),
			Side:       trade.SideBuy,
			Source:     a.Source(),
			Quantity:   a.Quantity(),
			Price:      a.UnitCost(),
			Fee:        zeroFee,
			ExecutedAt: a.AcquiredAt(),
		})
		if err != nil {
			continue
		}
		out = append(out, tr)
	}
	return out
}
