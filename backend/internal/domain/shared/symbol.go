// Package shared holds value objects used across the binancetracker domain.
//
// Value objects in this package are immutable, self-validating, and free of
// any framework or infrastructure concern. They form the vocabulary the rest
// of the domain speaks.
package shared

import (
	"errors"
	"strings"
)

// ErrInvalidSymbol is returned when constructing a Symbol with empty or
// non-conforming input.
var ErrInvalidSymbol = errors.New("invalid asset symbol")

// Symbol identifies a tradable asset (e.g. BTC, ETH, USDT). Symbols are
// always upper-case ASCII.
type Symbol string

// NewSymbol validates and normalises an asset symbol.
func NewSymbol(raw string) (Symbol, error) {
	trimmed := strings.TrimSpace(strings.ToUpper(raw))
	if trimmed == "" {
		return "", ErrInvalidSymbol
	}
	for _, r := range trimmed {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return "", ErrInvalidSymbol
		}
	}
	return Symbol(trimmed), nil
}

// String returns the canonical string representation of the symbol.
func (s Symbol) String() string { return string(s) }

// Equals reports whether two symbols refer to the same asset.
func (s Symbol) Equals(other Symbol) bool { return s == other }

// IsZero reports whether the symbol is the zero value.
func (s Symbol) IsZero() bool { return s == "" }
