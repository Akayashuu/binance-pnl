package queries

import (
	"context"

	"github.com/binancetracker/binancetracker/internal/application/ports"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/binancetracker/binancetracker/internal/domain/trade"
)

// ListTrades returns the unified history: real trades plus manual funds and
// other acquisitions surfaced as synthetic BUY rows so the user sees one
// chronological stream.
type ListTrades struct {
	trades       ports.TradeRepository
	acquisitions ports.AcquisitionRepository
	quote        shared.Symbol
}

func NewListTrades(
	trades ports.TradeRepository,
	acquisitions ports.AcquisitionRepository,
	quote shared.Symbol,
) *ListTrades {
	return &ListTrades{trades: trades, acquisitions: acquisitions, quote: quote}
}

func (uc *ListTrades) Execute(ctx context.Context, assetFilter string) ([]trade.Trade, error) {
	if assetFilter == "" {
		ts, err := uc.trades.ListAll(ctx)
		if err != nil {
			return nil, err
		}
		all, err := uc.acquisitions.ListAll(ctx)
		if err != nil {
			return nil, err
		}
		ts = append(ts, acquisitionsAsBuyTrades(all)...)
		sortTradesByTime(ts)
		return ts, nil
	}

	sym, err := parseSymbolOrEmpty(assetFilter)
	if err != nil {
		return nil, err
	}
	ts, err := uc.trades.ListByAsset(ctx, sym)
	if err != nil {
		return nil, err
	}
	filtered, err := uc.acquisitions.ListByAsset(ctx, sym)
	if err != nil {
		return nil, err
	}
	ts = append(ts, acquisitionsAsBuyTrades(filtered)...)
	sortTradesByTime(ts)
	return ts, nil
}
