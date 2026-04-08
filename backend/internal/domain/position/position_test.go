package position_test

import (
	"testing"
	"time"

	"github.com/binancetracker/binancetracker/internal/domain/position"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/binancetracker/binancetracker/internal/domain/trade"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustTrade(t *testing.T, id string, side trade.Side, qty, price string, when time.Time) trade.Trade {
	t.Helper()
	btc, _ := shared.NewSymbol("BTC")
	usdt, _ := shared.NewSymbol("USDT")
	tr, err := trade.New(trade.Params{
		ID:         id,
		Asset:      btc,
		Quote:      usdt,
		Side:       side,
		Quantity:   shared.MustQuantityFromString(qty),
		Price:      shared.MustMoneyFromString(price, "USDT"),
		Fee:        shared.MustMoneyFromString("0", "USDT"),
		ExecutedAt: when,
	})
	require.NoError(t, err)
	return tr
}

func TestPositionWeightedAverageCost(t *testing.T) {
	t.Parallel()
	btc, _ := shared.NewSymbol("BTC")
	usdt, _ := shared.NewSymbol("USDT")

	// Buy 1 BTC @ 50000, then 1 BTC @ 70000 → avg = 60000
	t1 := mustTrade(t, "1", trade.SideBuy, "1", "50000", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	t2 := mustTrade(t, "2", trade.SideBuy, "1", "70000", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))

	pos, err := position.Build(btc, usdt, []trade.Trade{t2, t1}) // unsorted on purpose
	require.NoError(t, err)

	assert.Equal(t, "2", pos.HeldQuantity().Decimal().String())
	assert.Equal(t, "60000", pos.AverageCost().Amount().String())
	assert.Equal(t, "120000", pos.TotalInvested().Amount().String())
	assert.Equal(t, "0", pos.RealizedPnL().Amount().String())
}

func TestPositionRealizedPnLOnSell(t *testing.T) {
	t.Parallel()
	btc, _ := shared.NewSymbol("BTC")
	usdt, _ := shared.NewSymbol("USDT")

	// Buy 2 BTC @ 50000 (avg = 50000), sell 1 BTC @ 60000 → realized = 10000
	t1 := mustTrade(t, "1", trade.SideBuy, "2", "50000", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	t2 := mustTrade(t, "2", trade.SideSell, "1", "60000", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))

	pos, err := position.Build(btc, usdt, []trade.Trade{t1, t2})
	require.NoError(t, err)

	assert.Equal(t, "1", pos.HeldQuantity().Decimal().String())
	assert.Equal(t, "50000", pos.AverageCost().Amount().String())
	assert.Equal(t, "10000", pos.RealizedPnL().Amount().String())
}

func TestPositionFeesIncreaseCostBasis(t *testing.T) {
	t.Parallel()
	btc, _ := shared.NewSymbol("BTC")
	usdt, _ := shared.NewSymbol("USDT")

	// Buy 1 BTC @ 50000 with 100 USDT fee → cost basis = 50100
	tr, err := trade.New(trade.Params{
		ID:         "1",
		Asset:      btc,
		Quote:      usdt,
		Side:       trade.SideBuy,
		Quantity:   shared.MustQuantityFromString("1"),
		Price:      shared.MustMoneyFromString("50000", "USDT"),
		Fee:        shared.MustMoneyFromString("100", "USDT"),
		ExecutedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	pos, err := position.Build(btc, usdt, []trade.Trade{tr})
	require.NoError(t, err)
	assert.Equal(t, "50100", pos.AverageCost().Amount().String())
	assert.Equal(t, "50100", pos.TotalInvested().Amount().String())
}

func TestPositionRejectsOverSell(t *testing.T) {
	t.Parallel()
	btc, _ := shared.NewSymbol("BTC")
	usdt, _ := shared.NewSymbol("USDT")

	t1 := mustTrade(t, "1", trade.SideBuy, "1", "50000", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	t2 := mustTrade(t, "2", trade.SideSell, "2", "60000", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))

	_, err := position.Build(btc, usdt, []trade.Trade{t1, t2})
	assert.ErrorIs(t, err, position.ErrSellExceedsHoldings)
}

func TestPositionEmptyTrades(t *testing.T) {
	t.Parallel()
	btc, _ := shared.NewSymbol("BTC")
	usdt, _ := shared.NewSymbol("USDT")
	pos, err := position.Build(btc, usdt, nil)
	require.NoError(t, err)
	assert.True(t, pos.HeldQuantity().IsZero())
	assert.Equal(t, 0, pos.TradeCount())
}
