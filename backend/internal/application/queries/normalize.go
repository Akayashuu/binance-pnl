package queries

import (
	"context"
	"sort"

	"github.com/binancetracker/binancetracker/internal/application/ports"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/binancetracker/binancetracker/internal/domain/trade"
)

// sortTradesByTime orders a slice in-place by ExecutedAt ascending. Used to
// merge real trades and synthetic acquisitions into a single chronological
// history.
func sortTradesByTime(in []trade.Trade) {
	sort.SliceStable(in, func(i, j int) bool {
		return in[i].ExecutedAt().Before(in[j].ExecutedAt())
	})
}

// normalizeTradesToQuote rewrites every trade so its price is denominated in
// `target`, using the current FX rate to bridge mismatched quotes.
//
// Why this exists: position.Build refuses to mix quotes (mixing EUR and USDT
// in one running cost basis would be nonsense). Without normalization, EUR
// fiat-buy trades are silently dropped from a USDT-quoted position and the
// user sees quantity = 0 even though the trades are visible in the history.
//
// We use the current rate, not the historical one. That's wrong for cost
// basis accuracy on old trades — but it's far better than ignoring the trade
// altogether. A future iteration can resolve historical FX via klines.
func normalizeTradesToQuote(
	ctx context.Context,
	fx ports.FxRateProvider,
	trades []trade.Trade,
	target shared.Symbol,
) []trade.Trade {
	out := make([]trade.Trade, 0, len(trades))
	for _, tr := range trades {
		if tr.Quote().Equals(target) {
			out = append(out, tr)
			continue
		}
		if fx == nil {
			continue
		}
		rate, err := fx.Rate(ctx, tr.Quote(), target)
		if err != nil {
			continue
		}
		newPrice, err := shared.NewMoney(tr.Price().Amount().Mul(rate), target)
		if err != nil {
			continue
		}
		fee := tr.Fee()
		if fee.Currency().Equals(tr.Quote()) {
			converted, err := shared.NewMoney(fee.Amount().Mul(rate), target)
			if err == nil {
				fee = converted
			}
		}
		normalized, err := trade.New(trade.Params{
			ID:         tr.ID(),
			Asset:      tr.Asset(),
			Quote:      target,
			Side:       tr.Side(),
			Source:     tr.Source(),
			Quantity:   tr.Quantity(),
			Price:      newPrice,
			Fee:        fee,
			ExecutedAt: tr.ExecutedAt(),
		})
		if err != nil {
			continue
		}
		out = append(out, normalized)
	}
	return out
}
