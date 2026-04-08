package shared

import (
	"errors"

	"github.com/shopspring/decimal"
)

// ErrNegativeQuantity is returned when a Quantity is constructed from a
// negative value.
var ErrNegativeQuantity = errors.New("quantity must be non-negative")

// Quantity represents an amount of an asset (e.g. 0.5 BTC). Quantities use
// fixed-point arithmetic and are never negative.
type Quantity struct {
	value decimal.Decimal
}

// NewQuantity returns a Quantity, rejecting negative values.
func NewQuantity(value decimal.Decimal) (Quantity, error) {
	if value.IsNegative() {
		return Quantity{}, ErrNegativeQuantity
	}
	return Quantity{value: value}, nil
}

// MustQuantityFromString builds a Quantity from a string. It panics on
// invalid input and is intended for tests and constants.
func MustQuantityFromString(raw string) Quantity {
	d, err := decimal.NewFromString(raw)
	if err != nil {
		panic(err)
	}
	q, err := NewQuantity(d)
	if err != nil {
		panic(err)
	}
	return q
}

// Decimal exposes the underlying value for read-only computation.
func (q Quantity) Decimal() decimal.Decimal { return q.value }

// IsZero reports whether the quantity is exactly zero.
func (q Quantity) IsZero() bool { return q.value.IsZero() }

// Add returns the sum of two quantities.
func (q Quantity) Add(other Quantity) Quantity {
	return Quantity{value: q.value.Add(other.value)}
}

// Sub subtracts other from q. It returns an error if the result would be
// negative — selling more than you hold is a domain violation.
func (q Quantity) Sub(other Quantity) (Quantity, error) {
	res := q.value.Sub(other.value)
	if res.IsNegative() {
		return Quantity{}, ErrNegativeQuantity
	}
	return Quantity{value: res}, nil
}
