package trade_test

import (
	"testing"
	"time"

	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/binancetracker/binancetracker/internal/domain/trade"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newValidParams() trade.Params {
	btc, _ := shared.NewSymbol("BTC")
	usdt, _ := shared.NewSymbol("USDT")
	return trade.Params{
		ID:         "binance-1",
		Asset:      btc,
		Quote:      usdt,
		Side:       trade.SideBuy,
		Quantity:   shared.MustQuantityFromString("0.5"),
		Price:      shared.MustMoneyFromString("60000", "USDT"),
		Fee:        shared.MustMoneyFromString("0.1", "USDT"),
		ExecutedAt: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
	}
}

func TestNewTradeValid(t *testing.T) {
	t.Parallel()
	tr, err := trade.New(newValidParams())
	require.NoError(t, err)
	assert.Equal(t, "binance-1", tr.ID())
	assert.Equal(t, trade.SideBuy, tr.Side())
	assert.Equal(t, "30000", tr.GrossValue().Amount().String())
}

func TestNewTradeRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		mutate func(*trade.Params)
		want   error
	}{
		{"empty id", func(p *trade.Params) { p.ID = "" }, trade.ErrInvalidTradeID},
		{"missing asset", func(p *trade.Params) { p.Asset = "" }, shared.ErrInvalidSymbol},
		{"missing quote", func(p *trade.Params) { p.Quote = "" }, shared.ErrInvalidSymbol},
		{"invalid side", func(p *trade.Params) { p.Side = "FOO" }, trade.ErrInvalidSide},
		{"zero quantity", func(p *trade.Params) { p.Quantity = shared.MustQuantityFromString("0") }, trade.ErrZeroQuantity},
		{"zero exec time", func(p *trade.Params) { p.ExecutedAt = time.Time{} }, trade.ErrInvalidExecutionTime},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := newValidParams()
			tc.mutate(&p)
			_, err := trade.New(p)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}
