package shared

import "errors"

// Source identifies the origin of a trade or acquisition. Spot/Convert/FiatBuy
// /Recurring carry their own price (reciprocal exchange). Deposit and
// EarnReward don't — their cost basis must be imputed from the spot price at
// the time of the event.
type Source string

const (
	SourceSpot       Source = "spot"
	SourceConvert    Source = "convert"
	SourceFiatBuy    Source = "fiat_buy"
	SourceRecurring  Source = "recurring"
	SourceDeposit    Source = "deposit"
	SourceEarnReward Source = "earn_reward"
)

var ErrInvalidSource = errors.New("invalid source")

func (s Source) IsValid() bool {
	switch s {
	case SourceSpot, SourceConvert, SourceFiatBuy, SourceRecurring,
		SourceDeposit, SourceEarnReward:
		return true
	}
	return false
}

func (s Source) String() string { return string(s) }

func ParseSource(raw string) (Source, error) {
	s := Source(raw)
	if !s.IsValid() {
		return "", ErrInvalidSource
	}
	return s, nil
}
