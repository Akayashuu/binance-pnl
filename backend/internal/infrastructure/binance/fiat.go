package binance

import (
	"context"
	"fmt"
	"time"

	bn "github.com/adshao/go-binance/v2"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/binancetracker/binancetracker/internal/domain/trade"
	"github.com/shopspring/decimal"
)

// FetchFiatBuysSince walks the user's "Buy Crypto" history (card / SEPA
// purchases) and turns each successful payment into a domain Trade.
//
// IMPORTANT: fiat purchases are denominated in the user's fiat currency
// (EUR, USD, GBP…) — NOT in the configured quote asset (typically USDT).
// We treat the FiatCurrency as the quote on the trade so that the price is
// stored faithfully. The position layer compares trade.Quote to the
// configured quote and ignores trades whose quote doesn't match — meaning
// fiat-denominated buys won't show up in the USDT-quoted dashboard unless
// the user runs an FX conversion. This is acceptable for v1; the trade row
// is still persisted so we can revisit valuation later.
//
// Pagination: Binance returns up to 500 rows per page; we walk pages until
// the response is empty. Window: 90 days max per call.
func (c *Client) FetchFiatBuysSince(ctx context.Context, since time.Time) ([]trade.Trade, error) {
	end := time.Now()
	if since.IsZero() {
		since = end.AddDate(-1, 0, 0) // 1 year of history on first sync
	}

	var out []trade.Trade
	const window = 90 * 24 * time.Hour
	for cursor := since; cursor.Before(end); cursor = cursor.Add(window) {
		windowEnd := cursor.Add(window)
		if windowEnd.After(end) {
			windowEnd = end
		}
		for page := int32(1); ; page++ {
			res, err := c.api.NewFiatPaymentsHistoryService().
				TransactionType(bn.TransactionTypeBuy).
				BeginTime(cursor.UnixMilli()).
				EndTime(windowEnd.UnixMilli()).
				Page(page).
				Rows(500).
				Do(ctx)
			if err != nil {
				return nil, fmt.Errorf("fiat payments [%s..%s] page %d: %w",
					cursor.Format(time.RFC3339), windowEnd.Format(time.RFC3339), page, err)
			}
			if len(res.Data) == 0 {
				break
			}
			for _, item := range res.Data {
				if item.Status != "Completed" && item.Status != "Successful" {
					continue
				}
				tr, ok, err := fiatBuyToTrade(item)
				if err != nil {
					return nil, fmt.Errorf("fiat order %s: %w", item.OrderNo, err)
				}
				if !ok {
					continue
				}
				out = append(out, tr)
			}
			if len(res.Data) < 500 {
				break
			}
		}
	}
	return out, nil
}

// fiatBuyToTrade maps a FiatPaymentsHistoryItem (a Buy Crypto purchase) to a
// domain Trade. The trade's quote currency is the FIAT currency used for the
// purchase (EUR, USD…), not the configured app quote.
func fiatBuyToTrade(item bn.FiatPaymentsHistoryItem) (trade.Trade, bool, error) {
	cryptoSym, err := shared.NewSymbol(item.CryptoCurrency)
	if err != nil {
		return trade.Trade{}, false, err
	}
	fiatSym, err := shared.NewSymbol(item.FiatCurrency)
	if err != nil {
		return trade.Trade{}, false, err
	}

	obtained, err := decimal.NewFromString(item.ObtainAmount)
	if err != nil {
		return trade.Trade{}, false, fmt.Errorf("obtain amount: %w", err)
	}
	if obtained.IsZero() {
		return trade.Trade{}, false, nil
	}

	price, err := decimal.NewFromString(item.Price)
	if err != nil {
		return trade.Trade{}, false, fmt.Errorf("price: %w", err)
	}

	fee, err := decimal.NewFromString(item.TotalFee)
	if err != nil {
		// Fee is optional in some payloads — fall back to zero.
		fee = decimal.Zero
	}

	qty, err := shared.NewQuantity(obtained)
	if err != nil {
		return trade.Trade{}, false, err
	}
	priceMoney, err := shared.NewMoney(price, fiatSym)
	if err != nil {
		return trade.Trade{}, false, err
	}
	feeMoney, err := shared.NewMoney(fee, fiatSym)
	if err != nil {
		return trade.Trade{}, false, err
	}

	tr, err := trade.New(trade.Params{
		ID:         "fiatbuy-" + item.OrderNo,
		Asset:      cryptoSym,
		Quote:      fiatSym,
		Side:       trade.SideBuy,
		Source:     shared.SourceFiatBuy,
		Quantity:   qty,
		Price:      priceMoney,
		Fee:        feeMoney,
		ExecutedAt: time.UnixMilli(item.CreateTime),
	})
	if err != nil {
		return trade.Trade{}, false, err
	}
	return tr, true, nil
}
