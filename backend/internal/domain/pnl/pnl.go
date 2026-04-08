// Package pnl computes profit and loss figures from positions and live
// market prices. It is a pure domain service: no I/O, no state.
package pnl

import (
	"github.com/binancetracker/binancetracker/internal/domain/position"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/shopspring/decimal"
)

// Result is the P&L view of a single position at a point in time.
type Result struct {
	Asset           shared.Symbol
	Quote           shared.Symbol
	HeldQuantity    shared.Quantity
	AverageCost     shared.Money
	CurrentPrice    shared.Money
	MarketValue     shared.Money // current_price * held_quantity
	CostBasis       shared.Money // average_cost * held_quantity
	UnrealizedPnL   shared.Money // market_value - cost_basis
	RealizedPnL     shared.Money // from the position
	TotalPnL        shared.Money // realized + unrealized
	UnrealizedPctBP int64        // pct change in basis points (1% = 100)
}

// Calculator turns a Position + current price into a P&L Result. It is
// stateless and safe for concurrent use.
type Calculator struct{}

// NewCalculator returns a default Calculator.
func NewCalculator() Calculator { return Calculator{} }

// Calculate produces a P&L Result for the given position at the given
// current price. The price's currency must match the position's quote.
//
// Returns shared.ErrCurrencyMismatch if currencies don't line up.
func (Calculator) Calculate(pos position.Position, currentPrice shared.Money) (Result, error) {
	if !currentPrice.Currency().Equals(pos.Quote()) {
		return Result{}, shared.ErrCurrencyMismatch
	}

	marketValue := currentPrice.MulQuantity(pos.HeldQuantity())
	costBasis := pos.AverageCost().MulQuantity(pos.HeldQuantity())

	unrealized, err := marketValue.Sub(costBasis)
	if err != nil {
		return Result{}, err
	}

	total, err := unrealized.Add(pos.RealizedPnL())
	if err != nil {
		return Result{}, err
	}

	return Result{
		Asset:           pos.Asset(),
		Quote:           pos.Quote(),
		HeldQuantity:    pos.HeldQuantity(),
		AverageCost:     pos.AverageCost(),
		CurrentPrice:    currentPrice,
		MarketValue:     marketValue,
		CostBasis:       costBasis,
		UnrealizedPnL:   unrealized,
		RealizedPnL:     pos.RealizedPnL(),
		TotalPnL:        total,
		UnrealizedPctBP: pctBasisPoints(unrealized.Amount(), costBasis.Amount()),
	}, nil
}

// pctBasisPoints returns (numerator/denominator) * 10000 as an int64,
// rounded toward zero. Returns 0 when the denominator is zero.
func pctBasisPoints(numerator, denominator decimal.Decimal) int64 {
	if denominator.IsZero() {
		return 0
	}
	bp := numerator.Div(denominator).Mul(decimal.NewFromInt(10000))
	return bp.IntPart()
}
