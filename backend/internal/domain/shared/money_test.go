package shared_test

import (
	"testing"

	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMoneyArithmetic(t *testing.T) {
	t.Parallel()

	a := shared.MustMoneyFromString("100.50", "USDT")
	b := shared.MustMoneyFromString("25.25", "USDT")

	sum, err := a.Add(b)
	require.NoError(t, err)
	assert.Equal(t, "125.75", sum.Amount().String())

	diff, err := a.Sub(b)
	require.NoError(t, err)
	assert.Equal(t, "75.25", diff.Amount().String())
}

func TestMoneyCurrencyMismatch(t *testing.T) {
	t.Parallel()
	usdt := shared.MustMoneyFromString("10", "USDT")
	eur := shared.MustMoneyFromString("10", "EUR")
	_, err := usdt.Add(eur)
	assert.ErrorIs(t, err, shared.ErrCurrencyMismatch)
}

func TestMoneyMulQuantity(t *testing.T) {
	t.Parallel()
	price := shared.MustMoneyFromString("60000", "USDT")
	qty := shared.MustQuantityFromString("0.5")
	total := price.MulQuantity(qty)
	assert.Equal(t, "30000", total.Amount().String())
	assert.Equal(t, "USDT", total.Currency().String())
}

func TestQuantitySubNegativeRejected(t *testing.T) {
	t.Parallel()
	q := shared.MustQuantityFromString("1")
	_, err := q.Sub(shared.MustQuantityFromString("2"))
	assert.ErrorIs(t, err, shared.ErrNegativeQuantity)
}

func TestNewQuantityNegative(t *testing.T) {
	t.Parallel()
	_, err := shared.NewQuantity(decimal.NewFromInt(-1))
	assert.ErrorIs(t, err, shared.ErrNegativeQuantity)
}
