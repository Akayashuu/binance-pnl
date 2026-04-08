package binance

import (
	"context"
	"fmt"
	"time"

	"github.com/binancetracker/binancetracker/internal/domain/acquisition"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/shopspring/decimal"
)

// FetchDepositsSince returns crypto deposits hitting the user's account after
// `since`. Each deposit becomes an Acquisition whose cost basis is the spot
// price of (coin, quote) at the moment of the deposit, fetched via /klines.
//
// Binance constraints: deposit history requires a startTime/endTime window of
// at most 90 days.
func (c *Client) FetchDepositsSince(ctx context.Context, since time.Time) ([]acquisition.Acquisition, error) {
	end := time.Now()
	if since.IsZero() {
		since = end.AddDate(-1, 0, 0)
	}

	var out []acquisition.Acquisition
	const window = 90 * 24 * time.Hour
	for cursor := since; cursor.Before(end); cursor = cursor.Add(window) {
		windowEnd := cursor.Add(window)
		if windowEnd.After(end) {
			windowEnd = end
		}
		res, err := c.api.NewListDepositsService().
			Status(1). // 1 = SUCCESS
			StartTime(cursor.UnixMilli()).
			EndTime(windowEnd.UnixMilli()).
			Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("deposits [%s..%s]: %w",
				cursor.Format(time.RFC3339), windowEnd.Format(time.RFC3339), err)
		}
		for _, d := range res {
			if d.Status != 1 {
				continue
			}
			coinSym, err := shared.NewSymbol(d.Coin)
			if err != nil {
				continue
			}
			qtyDec, err := decimal.NewFromString(d.Amount)
			if err != nil || qtyDec.IsZero() {
				continue
			}
			qty, err := shared.NewQuantity(qtyDec)
			if err != nil {
				continue
			}
			depositedAt := time.UnixMilli(d.InsertTime)

			unitCost, err := c.PriceAt(ctx, coinSym, c.cfg.QuoteAsset, depositedAt)
			if err != nil {
				return nil, fmt.Errorf("price for deposit %s@%s: %w",
					d.Coin, depositedAt.Format(time.RFC3339), err)
			}

			id := "deposit-" + d.TxID
			if d.TxID == "" {
				// Some deposits (internal transfers) lack a TxID. Fall back to
				// coin+timestamp+amount as a deterministic key.
				id = fmt.Sprintf("deposit-%s-%d-%s", d.Coin, d.InsertTime, d.Amount)
			}

			a, err := acquisition.New(acquisition.Params{
				ID:         id,
				Asset:      coinSym,
				Quote:      c.cfg.QuoteAsset,
				Source:     shared.SourceDeposit,
				Quantity:   qty,
				UnitCost:   unitCost,
				AcquiredAt: depositedAt,
			})
			if err != nil {
				return nil, fmt.Errorf("build acquisition for deposit %s: %w", id, err)
			}
			out = append(out, a)
		}
	}
	return out, nil
}

// PriceAt returns the spot close price of (asset, quote) at a given moment by
// reading the 1m kline that contains it (falls back to a 1h kline window if
// the 1m bucket is empty). If asset == quote, returns 1.
func (c *Client) PriceAt(ctx context.Context, asset, quote shared.Symbol, at time.Time) (shared.Money, error) {
	if asset.Equals(quote) {
		return shared.NewMoney(decimal.NewFromInt(1), quote)
	}
	pair := asset.String() + quote.String()
	startMs := at.UnixMilli()
	endMs := at.Add(2 * time.Minute).UnixMilli()

	klines, err := c.api.NewKlinesService().
		Symbol(pair).
		Interval("1m").
		StartTime(startMs).
		EndTime(endMs).
		Limit(1).
		Do(ctx)
	if err != nil {
		return shared.Money{}, err
	}
	if len(klines) == 0 {
		// No 1m kline at that exact moment — try a wider 1h window which is
		// always available historically.
		klines, err = c.api.NewKlinesService().
			Symbol(pair).
			Interval("1h").
			StartTime(at.Add(-time.Hour).UnixMilli()).
			EndTime(at.Add(time.Hour).UnixMilli()).
			Limit(2).
			Do(ctx)
		if err != nil {
			return shared.Money{}, err
		}
		if len(klines) == 0 {
			return shared.Money{}, fmt.Errorf("no kline for %s at %s", pair, at.Format(time.RFC3339))
		}
	}

	closeAmt, err := decimal.NewFromString(klines[0].Close)
	if err != nil {
		return shared.Money{}, fmt.Errorf("parse close price: %w", err)
	}
	return shared.NewMoney(closeAmt, quote)
}
