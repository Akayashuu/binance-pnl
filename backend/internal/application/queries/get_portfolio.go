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
	trades       ports.TradeRepository
	acquisitions ports.AcquisitionRepository
	prices       ports.PriceRepository
	fx           ports.FxRateProvider
	calc         pnl.Calculator
	quote        shared.Symbol
	now          func() time.Time
}

func NewGetPortfolio(
	trades ports.TradeRepository,
	acquisitions ports.AcquisitionRepository,
	prices ports.PriceRepository,
	fx ports.FxRateProvider,
	quote shared.Symbol,
) *GetPortfolio {
	return &GetPortfolio{
		trades:       trades,
		acquisitions: acquisitions,
		prices:       prices,
		fx:           fx,
		calc:         pnl.NewCalculator(),
		quote:        quote,
		now:          time.Now,
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

	return overview, nil
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
