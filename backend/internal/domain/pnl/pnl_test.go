package pnl_test

import (
	"testing"
	"time"

	"github.com/binancetracker/binancetracker/internal/domain/pnl"
	"github.com/binancetracker/binancetracker/internal/domain/position"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/binancetracker/binancetracker/internal/domain/trade"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildPosition(t *testing.T) position.Position {
	t.Helper()
	btc, _ := shared.NewSymbol("BTC")
	usdt, _ := shared.NewSymbol("USDT")

	tr, err := trade.New(trade.Params{
		ID:         "1",
		Asset:      btc,
		Quote:      usdt,
		Side:       trade.SideBuy,
		Quantity:   shared.MustQuantityFromString("2"),
		Price:      shared.MustMoneyFromString("50000", "USDT"),
		Fee:        shared.MustMoneyFromString("0", "USDT"),
		ExecutedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	pos, err := position.Build(btc, usdt, []trade.Trade{tr})
	require.NoError(t, err)
	return pos
}

func TestCalculator_Unrealized(t *testing.T) {
	t.Parallel()
	pos := buildPosition(t)
	calc := pnl.NewCalculator()

	res, err := calc.Calculate(pos, shared.MustMoneyFromString("60000", "USDT"))
	require.NoError(t, err)

	assert.Equal(t, "120000", res.MarketValue.Amount().String())
	assert.Equal(t, "100000", res.CostBasis.Amount().String())
	assert.Equal(t, "20000", res.UnrealizedPnL.Amount().String())
	assert.Equal(t, "20000", res.TotalPnL.Amount().String())
	assert.Equal(t, int64(2000), res.UnrealizedPctBP) // 20%
}

func TestCalculator_NegativeUnrealized(t *testing.T) {
	t.Parallel()
	pos := buildPosition(t)
	calc := pnl.NewCalculator()

	res, err := calc.Calculate(pos, shared.MustMoneyFromString("40000", "USDT"))
	require.NoError(t, err)

	assert.Equal(t, "-20000", res.UnrealizedPnL.Amount().String())
	assert.Equal(t, int64(-2000), res.UnrealizedPctBP) // -20%
}

func TestCalculator_CurrencyMismatch(t *testing.T) {
	t.Parallel()
	pos := buildPosition(t)
	calc := pnl.NewCalculator()

	_, err := calc.Calculate(pos, shared.MustMoneyFromString("60000", "EUR"))
	assert.ErrorIs(t, err, shared.ErrCurrencyMismatch)
}
