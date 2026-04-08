// Package trade models a single buy or sell executed on an exchange.
package trade

import (
	"errors"
	"time"

	"github.com/binancetracker/binancetracker/internal/domain/shared"
)

// Side identifies whether a trade is a buy or a sell.
type Side string

const (
	// SideBuy represents the acquisition of a base asset.
	SideBuy Side = "BUY"
	// SideSell represents the disposal of a base asset.
	SideSell Side = "SELL"
)

// IsValid reports whether the side is one of the recognised values.
func (s Side) IsValid() bool { return s == SideBuy || s == SideSell }

// Domain errors.
var (
	ErrInvalidTradeID       = errors.New("trade id is required")
	ErrInvalidSide          = errors.New("trade side must be BUY or SELL")
	ErrInvalidExecutionTime = errors.New("execution time is required")
	ErrZeroQuantity         = errors.New("trade quantity must be greater than zero")
	ErrFeeCurrencyRequired  = errors.New("fee currency must be set")
)

// Trade is an immutable record of an executed transaction. Corrections are
// modelled as compensating trades — never mutate.
type Trade struct {
	id         string
	asset      shared.Symbol
	quote      shared.Symbol
	side       Side
	source     shared.Source
	quantity   shared.Quantity
	price      shared.Money
	fee        shared.Money
	executedAt time.Time
}

// Params is the constructor input for Trade.
type Params struct {
	ID         string
	Asset      shared.Symbol
	Quote      shared.Symbol
	Side       Side
	Source     shared.Source
	Quantity   shared.Quantity
	Price      shared.Money
	Fee        shared.Money
	ExecutedAt time.Time
}

// New constructs a validated Trade.
func New(p Params) (Trade, error) {
	if p.ID == "" {
		return Trade{}, ErrInvalidTradeID
	}
	if p.Asset.IsZero() || p.Quote.IsZero() {
		return Trade{}, shared.ErrInvalidSymbol
	}
	if !p.Side.IsValid() {
		return Trade{}, ErrInvalidSide
	}
	if p.Quantity.IsZero() {
		return Trade{}, ErrZeroQuantity
	}
	if p.ExecutedAt.IsZero() {
		return Trade{}, ErrInvalidExecutionTime
	}
	if p.Fee.Currency().IsZero() {
		return Trade{}, ErrFeeCurrencyRequired
	}
	src := p.Source
	if src == "" {
		src = shared.SourceSpot
	}
	if !src.IsValid() {
		return Trade{}, shared.ErrInvalidSource
	}
	return Trade{
		id:         p.ID,
		asset:      p.Asset,
		quote:      p.Quote,
		side:       p.Side,
		source:     src,
		quantity:   p.Quantity,
		price:      p.Price,
		fee:        p.Fee,
		executedAt: p.ExecutedAt,
	}, nil
}

// Accessors

// ID returns the unique identifier (typically the exchange trade id).
func (t Trade) ID() string { return t.id }

// Asset returns the base asset symbol traded.
func (t Trade) Asset() shared.Symbol { return t.asset }

// Quote returns the quote currency symbol.
func (t Trade) Quote() shared.Symbol { return t.quote }

// Side returns BUY or SELL.
func (t Trade) Side() Side { return t.side }

// Source returns the origin of the trade (spot, convert, fiat_buy, recurring).
func (t Trade) Source() shared.Source { return t.source }

// Quantity returns the quantity of the base asset traded.
func (t Trade) Quantity() shared.Quantity { return t.quantity }

// Price returns the unit price of the base asset in the quote currency.
func (t Trade) Price() shared.Money { return t.price }

// Fee returns the exchange fee charged.
func (t Trade) Fee() shared.Money { return t.fee }

// ExecutedAt returns when the trade was executed.
func (t Trade) ExecutedAt() time.Time { return t.executedAt }

// GrossValue returns price * quantity in the quote currency, ignoring fees.
func (t Trade) GrossValue() shared.Money {
	return t.price.MulQuantity(t.quantity)
}
