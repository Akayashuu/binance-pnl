package binance

import (
	"context"
	"fmt"

	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/shopspring/decimal"
)

// Kline is a single candlestick data point (OHLCV).
type Kline struct {
	OpenTime int64           `json:"open_time"`
	Open     decimal.Decimal `json:"open"`
	High     decimal.Decimal `json:"high"`
	Low      decimal.Decimal `json:"low"`
	Close    decimal.Decimal `json:"close"`
	Volume   decimal.Decimal `json:"volume"`
}

// FetchKlines returns historical close prices for the given asset in the
// configured quote currency. Interval should be a Binance interval string
// (e.g. "1h", "4h", "1d"). Limit caps the number of klines returned (max 1000).
func (c *Client) FetchKlines(ctx context.Context, asset shared.Symbol, interval string, limit int) ([]Kline, error) {
	if limit <= 0 || limit > 1000 {
		limit = 90
	}

	pair := asset.String() + c.cfg.QuoteAsset.String()
	res, err := c.api.NewKlinesService().
		Symbol(pair).
		Interval(interval).
		Limit(limit).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("klines %s: %w", pair, err)
	}

	out := make([]Kline, 0, len(res))
	for _, k := range res {
		openAmt, _ := decimal.NewFromString(k.Open)
		highAmt, _ := decimal.NewFromString(k.High)
		lowAmt, _ := decimal.NewFromString(k.Low)
		closeAmt, err := decimal.NewFromString(k.Close)
		if err != nil {
			continue
		}
		volAmt, _ := decimal.NewFromString(k.Volume)
		out = append(out, Kline{
			OpenTime: k.OpenTime,
			Open:     openAmt,
			High:     highAmt,
			Low:      lowAmt,
			Close:    closeAmt,
			Volume:   volAmt,
		})
	}
	return out, nil
}
