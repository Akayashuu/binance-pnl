// Package asset models a tradable cryptocurrency asset.
package asset

import (
	"errors"

	"github.com/binancetracker/binancetracker/internal/domain/shared"
)

// ErrInvalidAsset is returned when an Asset cannot be constructed.
var ErrInvalidAsset = errors.New("invalid asset")

// Asset is a uniquely identifiable cryptocurrency on the exchange.
//
// It is intentionally minimal — display name and symbol only. Market data
// (prices) lives outside the aggregate.
type Asset struct {
	symbol shared.Symbol
	name   string
}

// New constructs an Asset.
func New(symbol shared.Symbol, name string) (Asset, error) {
	if symbol.IsZero() {
		return Asset{}, ErrInvalidAsset
	}
	if name == "" {
		name = symbol.String()
	}
	return Asset{symbol: symbol, name: name}, nil
}

// Symbol returns the asset symbol.
func (a Asset) Symbol() shared.Symbol { return a.symbol }

// Name returns the human-friendly name.
func (a Asset) Name() string { return a.name }
