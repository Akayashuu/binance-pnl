package queries

import (
	"context"
	"fmt"

	"github.com/binancetracker/binancetracker/internal/application/ports"
	"github.com/binancetracker/binancetracker/internal/domain/acquisition"
	"github.com/binancetracker/binancetracker/internal/domain/pnl"
	"github.com/binancetracker/binancetracker/internal/domain/position"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/binancetracker/binancetracker/internal/domain/trade"
	"github.com/shopspring/decimal"
)

// LotView is a per-acquisition snapshot enriched with current price + P&L,
// ready for the HTTP DTO. It lives in the application layer (not the domain)
// because it carries valuation derived from a price feed snapshot.
type LotView struct {
	AcquisitionID     string
	Source            shared.Source
	AcquiredAt        string // RFC3339 — formatted at the boundary
	OriginalQuantity  decimal.Decimal
	RemainingQuantity decimal.Decimal
	UnitCost          decimal.Decimal
	CostBasis         decimal.Decimal
	CurrentPrice      decimal.Decimal
	CurrentValue      decimal.Decimal
	UnrealizedPnL     decimal.Decimal
	UnrealizedPnLPct  decimal.Decimal
}

// TradeView is a single transaction enriched with the P&L it would represent
// if held to today's price. Two flavours:
//
//   - Naïve delta (DeltaTotal/DeltaPct): treats every BUY as if the full
//     bought quantity were still held. Answers "on this purchase, how much
//     have I gained since?". Sells get a zero delta.
//   - FIFO-aware (RemainingQty/RemainingPnL): how much of this trade survived
//     subsequent sells under FIFO accounting, and the P&L on what remains.
//     Answers "of this purchase, how much is still in my pocket and how is
//     it doing?".
type TradeView struct {
	Trade           trade.Trade
	CurrentPrice    shared.Money
	DeltaPerUnit    decimal.Decimal // currentPrice - tradePrice (in trade.Quote)
	DeltaTotal      decimal.Decimal // DeltaPerUnit * tradeQuantity
	DeltaPct        decimal.Decimal // (currentPrice - tradePrice) / tradePrice * 100
	RemainingQty    decimal.Decimal // surviving quantity after FIFO consumption
	RemainingPnL    decimal.Decimal // DeltaPerUnit * RemainingQty
}

// AssetDetail is the read model for the per-asset drill-down view.
type AssetDetail struct {
	PnL          pnl.Result
	Trades       []trade.Trade
	TradeViews   []TradeView
	Acquisitions []acquisition.Acquisition
	Lots         []LotView
}

type GetAssetDetail struct {
	trades       ports.TradeRepository
	acquisitions ports.AcquisitionRepository
	prices       ports.PriceRepository
	fx           ports.FxRateProvider
	calc         pnl.Calculator
	quote        shared.Symbol
}

func NewGetAssetDetail(
	trades ports.TradeRepository,
	acquisitions ports.AcquisitionRepository,
	prices ports.PriceRepository,
	fx ports.FxRateProvider,
	quote shared.Symbol,
) *GetAssetDetail {
	return &GetAssetDetail{
		trades:       trades,
		acquisitions: acquisitions,
		prices:       prices,
		fx:           fx,
		calc:         pnl.NewCalculator(),
		quote:        quote,
	}
}

// Execute computes the detail for the given asset symbol.
func (uc *GetAssetDetail) Execute(ctx context.Context, rawSymbol string) (AssetDetail, error) {
	sym, err := shared.NewSymbol(rawSymbol)
	if err != nil {
		return AssetDetail{}, err
	}

	tradesOnly, err := uc.trades.ListByAsset(ctx, sym)
	if err != nil {
		return AssetDetail{}, fmt.Errorf("list trades: %w", err)
	}

	acqs, err := uc.acquisitions.ListByAsset(ctx, sym)
	if err != nil {
		return AssetDetail{}, fmt.Errorf("list acquisitions: %w", err)
	}

	// rawTrades is the list shown in the history table: real trades + manual
	// funds and deposits surfaced as synthetic BUY rows so the user sees
	// everything in one chronological view.
	rawTrades := append([]trade.Trade(nil), tradesOnly...)
	rawTrades = append(rawTrades, acquisitionsAsBuyTrades(acqs)...)
	sortTradesByTime(rawTrades)

	// Normalize trades into the position's quote currency. Without this,
	// EUR fiat-buys would be silently dropped from a USDT position and the
	// user would see quantity = 0 even with visible buys in history.
	trades := normalizeTradesToQuote(ctx, uc.fx, rawTrades, uc.quote)

	pos, err := position.Build(sym, uc.quote, trades)
	if err != nil {
		return AssetDetail{}, err
	}

	price, _, err := uc.prices.Latest(ctx, sym)
	if err != nil {
		price = pos.AverageCost()
	}

	result, err := uc.calc.Calculate(pos, price)
	if err != nil {
		return AssetDetail{}, err
	}

	lots := position.BuildLots(position.LotsInput{
		Asset:        sym,
		Quote:        uc.quote,
		Trades:       trades,
		Acquisitions: acqs,
	})

	current := price.Amount()
	views := make([]LotView, 0, len(lots))
	// Index lots by acquisition ID so the per-trade view can look up
	// "how much of this trade survived FIFO consumption" in O(1).
	lotByAcqID := make(map[string]position.Lot, len(lots))
	for _, l := range lots {
		lotByAcqID[l.AcquisitionID] = l
		views = append(views, LotView{
			AcquisitionID:     l.AcquisitionID,
			Source:            l.Source,
			AcquiredAt:        l.AcquiredAt.UTC().Format("2006-01-02T15:04:05Z"),
			OriginalQuantity:  l.OriginalQuantity,
			RemainingQuantity: l.RemainingQuantity,
			UnitCost:          l.UnitCost,
			CostBasis:         l.CostBasis(),
			CurrentPrice:      current,
			CurrentValue:      l.CurrentValue(current),
			UnrealizedPnL:     l.UnrealizedPnL(current),
			UnrealizedPnLPct:  l.UnrealizedPnLPct(current),
		})
	}

	// Index normalized trades by ID so the per-trade view can keep the
	// original trade row (in its native currency, e.g. EUR) for display while
	// using the normalized version for P&L math.
	normalizedByID := make(map[string]trade.Trade, len(trades))
	for _, tr := range trades {
		normalizedByID[tr.ID()] = tr
	}

	tradeViews := make([]TradeView, 0, len(rawTrades))
	for _, tr := range rawTrades {
		var (
			deltaPerUnit decimal.Decimal
			deltaTotal   decimal.Decimal
			deltaPct     decimal.Decimal
		)
		if normalized, ok := normalizedByID[tr.ID()]; ok && tr.Side() == trade.SideBuy {
			deltaPerUnit = current.Sub(normalized.Price().Amount())
			deltaTotal = deltaPerUnit.Mul(normalized.Quantity().Decimal())
			if !normalized.Price().Amount().IsZero() {
				deltaPct = deltaPerUnit.Div(normalized.Price().Amount()).Mul(decimal.NewFromInt(100))
			}
		}

		var (
			remainingQty decimal.Decimal
			remainingPnL decimal.Decimal
		)
		if l, ok := lotByAcqID[tr.ID()]; ok {
			remainingQty = l.RemainingQuantity
			remainingPnL = deltaPerUnit.Mul(remainingQty)
		}

		tradeViews = append(tradeViews, TradeView{
			Trade:        tr,
			CurrentPrice: price,
			DeltaPerUnit: deltaPerUnit,
			DeltaTotal:   deltaTotal,
			DeltaPct:     deltaPct,
			RemainingQty: remainingQty,
			RemainingPnL: remainingPnL,
		})
	}

	return AssetDetail{
		PnL:          result,
		Trades:       rawTrades,
		TradeViews:   tradeViews,
		Acquisitions: acqs,
		Lots:         views,
	}, nil
}

// parseSymbolOrEmpty is shared between queries; we keep it here to avoid an
// extra package for one helper.
func parseSymbolOrEmpty(raw string) (shared.Symbol, error) {
	return shared.NewSymbol(raw)
}
