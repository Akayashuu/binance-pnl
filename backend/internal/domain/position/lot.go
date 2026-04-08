package position

import (
	"sort"
	"time"

	"github.com/binancetracker/binancetracker/internal/domain/acquisition"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/binancetracker/binancetracker/internal/domain/trade"
	"github.com/shopspring/decimal"
)

// Lot is a single open acquisition. Each one tracks where it came from and
// how much survived FIFO consumption by sells, so the per-acquisition P&L
// view can show "this purchase has gained X" independently of the AVCO
// aggregate.
type Lot struct {
	AcquisitionID     string
	Asset             shared.Symbol
	Quote             shared.Symbol
	Source            shared.Source
	AcquiredAt        time.Time
	OriginalQuantity  decimal.Decimal
	RemainingQuantity decimal.Decimal
	UnitCost          decimal.Decimal
}

func (l Lot) CostBasis() decimal.Decimal {
	return l.RemainingQuantity.Mul(l.UnitCost)
}

func (l Lot) CurrentValue(currentPrice decimal.Decimal) decimal.Decimal {
	return l.RemainingQuantity.Mul(currentPrice)
}

func (l Lot) UnrealizedPnL(currentPrice decimal.Decimal) decimal.Decimal {
	return currentPrice.Sub(l.UnitCost).Mul(l.RemainingQuantity)
}

// UnrealizedPnLPct returns 0 when cost basis is 0 (free coin from a reward —
// percentage gain is undefined).
func (l Lot) UnrealizedPnLPct(currentPrice decimal.Decimal) decimal.Decimal {
	cost := l.CostBasis()
	if cost.IsZero() {
		return decimal.Zero
	}
	return l.UnrealizedPnL(currentPrice).Div(cost).Mul(decimal.NewFromInt(100))
}

type LotsInput struct {
	Asset        shared.Symbol
	Quote        shared.Symbol
	Trades       []trade.Trade
	Acquisitions []acquisition.Acquisition
}

// BuildLots replays trades + acquisitions in chronological order and returns
// the lots still open today. SELLs consume the oldest lot first (FIFO).
// Trades whose quote doesn't match in.Quote are skipped — they cannot be
// valued without an FX layer (e.g. EUR fiat-buy when the app quote is USDT).
// An over-sell (more than held) is silently tolerated so the rest of the view
// stays usable in the face of incomplete imports.
func BuildLots(in LotsInput) []Lot {
	type entry struct {
		t       time.Time
		asLot   Lot
		isSell  bool
		sellQty decimal.Decimal
	}

	events := make([]entry, 0, len(in.Trades)+len(in.Acquisitions))

	for _, tr := range in.Trades {
		if !tr.Asset().Equals(in.Asset) || !tr.Quote().Equals(in.Quote) {
			continue
		}
		switch tr.Side() {
		case trade.SideBuy:
			events = append(events, entry{
				t: tr.ExecutedAt(),
				asLot: Lot{
					AcquisitionID:     tr.ID(),
					Asset:             in.Asset,
					Quote:             in.Quote,
					Source:            tr.Source(),
					AcquiredAt:        tr.ExecutedAt(),
					OriginalQuantity:  tr.Quantity().Decimal(),
					RemainingQuantity: tr.Quantity().Decimal(),
					UnitCost:          tr.Price().Amount(),
				},
			})
		case trade.SideSell:
			events = append(events, entry{
				t:       tr.ExecutedAt(),
				isSell:  true,
				sellQty: tr.Quantity().Decimal(),
			})
		}
	}

	for _, a := range in.Acquisitions {
		if !a.Asset().Equals(in.Asset) || !a.Quote().Equals(in.Quote) {
			continue
		}
		events = append(events, entry{
			t: a.AcquiredAt(),
			asLot: Lot{
				AcquisitionID:     a.ID(),
				Asset:             in.Asset,
				Quote:             in.Quote,
				Source:            a.Source(),
				AcquiredAt:        a.AcquiredAt(),
				OriginalQuantity:  a.Quantity().Decimal(),
				RemainingQuantity: a.Quantity().Decimal(),
				UnitCost:          a.UnitCost().Amount(),
			},
		})
	}

	sort.SliceStable(events, func(i, j int) bool {
		return events[i].t.Before(events[j].t)
	})

	var openLots []Lot
	for _, e := range events {
		if !e.isSell {
			openLots = append(openLots, e.asLot)
			continue
		}
		remaining := e.sellQty
		for i := range openLots {
			if remaining.IsZero() {
				break
			}
			if openLots[i].RemainingQuantity.IsZero() {
				continue
			}
			if openLots[i].RemainingQuantity.GreaterThanOrEqual(remaining) {
				openLots[i].RemainingQuantity = openLots[i].RemainingQuantity.Sub(remaining)
				remaining = decimal.Zero
			} else {
				remaining = remaining.Sub(openLots[i].RemainingQuantity)
				openLots[i].RemainingQuantity = decimal.Zero
			}
		}
	}

	out := openLots[:0]
	for _, l := range openLots {
		if l.RemainingQuantity.IsZero() {
			continue
		}
		out = append(out, l)
	}
	return out
}
