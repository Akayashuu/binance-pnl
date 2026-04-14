// Package position computes the running cost basis and realized P&L for a
// single asset given a chronological list of trades.
//
// The cost basis model is **weighted average** (a.k.a. AVCO):
//
//   - On a BUY:  new_avg = (old_qty * old_avg + buy_qty * buy_price) / (old_qty + buy_qty)
//     held quantity increases by buy_qty.
//   - On a SELL: realized_pnl += sell_qty * (sell_price - avg_cost)
//     held quantity decreases by sell_qty; avg_cost is unchanged.
//
// Fees are added to cost basis on buys and subtracted from proceeds on sells
// when the fee is paid in the quote currency. Fees paid in other assets are
// ignored for v1 — they could be modelled as separate disposals later.
package position

import (
	"errors"
	"sort"

	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/binancetracker/binancetracker/internal/domain/trade"
	"github.com/shopspring/decimal"
)

// ErrSellExceedsHoldings indicates a sell trade attempted to dispose of more
// of the asset than was currently held — typically a sign of missing buy
// trades or a wrongly ordered import.
var ErrSellExceedsHoldings = errors.New("sell quantity exceeds current holdings")

// Position is the aggregated state of an asset derived from a sequence of
// trades. It is recomputed on demand rather than persisted directly.
type Position struct {
	asset        shared.Symbol
	quote        shared.Symbol
	heldQuantity shared.Quantity
	// avgCost is the cost basis per unit of asset, in quote currency.
	avgCost shared.Money
	// totalInvested is the absolute amount of quote currency that has flowed
	// into the position (sum of buy gross values + buy fees in quote).
	totalInvested shared.Money
	realizedPnL   shared.Money
	tradeCount    int
}

// Build computes the Position resulting from applying the given trades in
// chronological order. The trades may be passed in any order — Build sorts
// them defensively.
func Build(asset shared.Symbol, quote shared.Symbol, trades []trade.Trade) (Position, error) {
	pos := Position{
		asset:         asset,
		quote:         quote,
		heldQuantity:  shared.MustQuantityFromString("0"),
		avgCost:       shared.ZeroMoney(quote),
		totalInvested: shared.ZeroMoney(quote),
		realizedPnL:   shared.ZeroMoney(quote),
	}

	sorted := make([]trade.Trade, len(trades))
	copy(sorted, trades)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].ExecutedAt().Before(sorted[j].ExecutedAt())
	})

	for _, tr := range sorted {
		if !tr.Asset().Equals(asset) {
			continue
		}
		if !tr.Quote().Equals(quote) {
			// Mixing quote currencies on the same asset is unsupported in v1.
			continue
		}
		if err := pos.apply(tr); err != nil {
			return Position{}, err
		}
		pos.tradeCount++
	}

	return pos, nil
}

func (p *Position) apply(tr trade.Trade) error {
	switch tr.Side() {
	case trade.SideBuy:
		return p.applyBuy(tr)
	case trade.SideSell:
		return p.applySell(tr)
	default:
		return trade.ErrInvalidSide
	}
}

func (p *Position) applyBuy(tr trade.Trade) error {
	oldQty := p.heldQuantity.Decimal()
	buyQty := tr.Quantity().Decimal()
	newQty := oldQty.Add(buyQty)

	gross := tr.GrossValue() // price * qty in quote
	feeInQuote := decimal.Zero
	if tr.Fee().Currency().Equals(p.quote) {
		feeInQuote = tr.Fee().Amount()
	}
	totalCost := gross.Amount().Add(feeInQuote)

	// new_avg = (old_qty*old_avg + total_cost) / new_qty
	oldBasis := oldQty.Mul(p.avgCost.Amount())
	newBasis := oldBasis.Add(totalCost)
	if newQty.IsZero() {
		// Defensive: should not happen because buyQty > 0 enforced by Trade.
		return trade.ErrZeroQuantity
	}
	newAvg := newBasis.Div(newQty)

	heldQty, err := shared.NewQuantity(newQty)
	if err != nil {
		return err
	}
	avg, err := shared.NewMoney(newAvg, p.quote)
	if err != nil {
		return err
	}
	invested, err := shared.NewMoney(p.totalInvested.Amount().Add(totalCost), p.quote)
	if err != nil {
		return err
	}

	p.heldQuantity = heldQty
	p.avgCost = avg
	p.totalInvested = invested
	return nil
}

func (p *Position) applySell(tr trade.Trade) error {
	sellQty := tr.Quantity().Decimal()
	if sellQty.GreaterThan(p.heldQuantity.Decimal()) {
		return ErrSellExceedsHoldings
	}

	proceeds := tr.GrossValue().Amount()
	if tr.Fee().Currency().Equals(p.quote) {
		proceeds = proceeds.Sub(tr.Fee().Amount())
	}

	costRemoved := sellQty.Mul(p.avgCost.Amount())
	realized := proceeds.Sub(costRemoved)

	newQty, err := p.heldQuantity.Sub(tr.Quantity())
	if err != nil {
		return err
	}

	newRealized, err := shared.NewMoney(p.realizedPnL.Amount().Add(realized), p.quote)
	if err != nil {
		return err
	}

	// Reduce totalInvested by the cost basis of the sold portion so that
	// "Total Invested" reflects capital still at work, not lifetime buys.
	newInvested, err := shared.NewMoney(p.totalInvested.Amount().Sub(costRemoved), p.quote)
	if err != nil {
		return err
	}

	p.heldQuantity = newQty
	p.realizedPnL = newRealized
	p.totalInvested = newInvested
	// Average cost is unchanged on sells under AVCO.
	return nil
}

// Asset returns the base asset symbol.
func (p Position) Asset() shared.Symbol { return p.asset }

// Quote returns the quote currency.
func (p Position) Quote() shared.Symbol { return p.quote }

// HeldQuantity returns the quantity of the asset currently held.
func (p Position) HeldQuantity() shared.Quantity { return p.heldQuantity }

// AverageCost returns the weighted average cost per unit of asset.
func (p Position) AverageCost() shared.Money { return p.avgCost }

// TotalInvested returns the gross amount of quote currency deployed into
// the position over its lifetime.
func (p Position) TotalInvested() shared.Money { return p.totalInvested }

// RealizedPnL returns realized profit/loss in the quote currency.
func (p Position) RealizedPnL() shared.Money { return p.realizedPnL }

// TradeCount returns the number of trades that contributed to this position.
func (p Position) TradeCount() int { return p.tradeCount }
