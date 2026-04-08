package shared

import (
	"errors"

	"github.com/shopspring/decimal"
)

// ErrCurrencyMismatch is returned when arithmetic is attempted between Money
// values denominated in different currencies.
var ErrCurrencyMismatch = errors.New("currency mismatch")

// Money is a monetary amount in a specific currency (which is itself a
// Symbol — for crypto contexts the currency is e.g. USDT, EUR, BTC).
//
// Money values are immutable. Negative amounts are allowed (e.g. realized
// losses) — only Quantity is constrained to be non-negative.
type Money struct {
	amount   decimal.Decimal
	currency Symbol
}

// NewMoney builds a Money value. It returns an error if the currency is
// invalid.
func NewMoney(amount decimal.Decimal, currency Symbol) (Money, error) {
	if currency.IsZero() {
		return Money{}, ErrInvalidSymbol
	}
	return Money{amount: amount, currency: currency}, nil
}

// MustMoneyFromString builds a Money value from a decimal string. It panics
// on invalid input — intended for tests and constants.
func MustMoneyFromString(amount string, currency string) Money {
	d, err := decimal.NewFromString(amount)
	if err != nil {
		panic(err)
	}
	sym, err := NewSymbol(currency)
	if err != nil {
		panic(err)
	}
	m, err := NewMoney(d, sym)
	if err != nil {
		panic(err)
	}
	return m
}

// ZeroMoney returns a zero amount in the given currency.
func ZeroMoney(currency Symbol) Money {
	return Money{amount: decimal.Zero, currency: currency}
}

// Amount exposes the underlying decimal value.
func (m Money) Amount() decimal.Decimal { return m.amount }

// Currency returns the symbol the amount is denominated in.
func (m Money) Currency() Symbol { return m.currency }

// IsZero reports whether the amount is exactly zero.
func (m Money) IsZero() bool { return m.amount.IsZero() }

// Add returns the sum of two money values, requiring matching currencies.
func (m Money) Add(other Money) (Money, error) {
	if !m.currency.Equals(other.currency) {
		return Money{}, ErrCurrencyMismatch
	}
	return Money{amount: m.amount.Add(other.amount), currency: m.currency}, nil
}

// Sub returns m - other, requiring matching currencies.
func (m Money) Sub(other Money) (Money, error) {
	if !m.currency.Equals(other.currency) {
		return Money{}, ErrCurrencyMismatch
	}
	return Money{amount: m.amount.Sub(other.amount), currency: m.currency}, nil
}

// MulQuantity multiplies a unit price by a quantity to get a total amount.
// Useful for cost basis and valuation calculations.
func (m Money) MulQuantity(q Quantity) Money {
	return Money{amount: m.amount.Mul(q.Decimal()), currency: m.currency}
}

// MulDecimal scales the monetary amount by a unit-less factor.
func (m Money) MulDecimal(factor decimal.Decimal) Money {
	return Money{amount: m.amount.Mul(factor), currency: m.currency}
}

// DivDecimal divides the monetary amount by a non-zero factor. Callers must
// ensure factor is not zero.
func (m Money) DivDecimal(factor decimal.Decimal) Money {
	return Money{amount: m.amount.Div(factor), currency: m.currency}
}
