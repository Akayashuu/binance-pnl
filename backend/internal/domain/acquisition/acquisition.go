// Package acquisition models the receipt of an asset that did NOT happen via
// an exchange order — typically a crypto deposit or an Earn reward. The cost
// basis is imputed from the spot price at the moment the asset hit the account.
package acquisition

import (
	"errors"
	"time"

	"github.com/binancetracker/binancetracker/internal/domain/shared"
)

var (
	ErrInvalidID             = errors.New("acquisition id is required")
	ErrInvalidAcquisitionAt  = errors.New("acquisition time is required")
	ErrZeroQuantity          = errors.New("acquisition quantity must be greater than zero")
	ErrCostBasisCurrencyMiss = errors.New("cost basis currency must be set")
)

type Acquisition struct {
	id         string
	asset      shared.Symbol
	quote      shared.Symbol
	source     shared.Source
	quantity   shared.Quantity
	unitCost   shared.Money
	acquiredAt time.Time
}

type Params struct {
	ID         string
	Asset      shared.Symbol
	Quote      shared.Symbol
	Source     shared.Source
	Quantity   shared.Quantity
	UnitCost   shared.Money
	AcquiredAt time.Time
}

func New(p Params) (Acquisition, error) {
	if p.ID == "" {
		return Acquisition{}, ErrInvalidID
	}
	if p.Asset.IsZero() || p.Quote.IsZero() {
		return Acquisition{}, shared.ErrInvalidSymbol
	}
	if !p.Source.IsValid() {
		return Acquisition{}, shared.ErrInvalidSource
	}
	if p.Quantity.IsZero() {
		return Acquisition{}, ErrZeroQuantity
	}
	if p.AcquiredAt.IsZero() {
		return Acquisition{}, ErrInvalidAcquisitionAt
	}
	if p.UnitCost.Currency().IsZero() {
		return Acquisition{}, ErrCostBasisCurrencyMiss
	}
	return Acquisition{
		id:         p.ID,
		asset:      p.Asset,
		quote:      p.Quote,
		source:     p.Source,
		quantity:   p.Quantity,
		unitCost:   p.UnitCost,
		acquiredAt: p.AcquiredAt,
	}, nil
}

func (a Acquisition) ID() string              { return a.id }
func (a Acquisition) Asset() shared.Symbol    { return a.asset }
func (a Acquisition) Quote() shared.Symbol    { return a.quote }
func (a Acquisition) Source() shared.Source   { return a.source }
func (a Acquisition) Quantity() shared.Quantity { return a.quantity }
func (a Acquisition) UnitCost() shared.Money  { return a.unitCost }
func (a Acquisition) AcquiredAt() time.Time   { return a.acquiredAt }

func (a Acquisition) GrossValue() shared.Money {
	return a.unitCost.MulQuantity(a.quantity)
}
