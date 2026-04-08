package binance

import (
	"context"
	"fmt"
	"strconv"
	"time"

	bn "github.com/adshao/go-binance/v2"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/binancetracker/binancetracker/internal/domain/trade"
	"github.com/shopspring/decimal"
)

// FetchConvertSince walks Binance Convert history. Cross-converts (neither
// leg in the configured quote) are skipped — they would need an extra price
// lookup to be valued. Window is capped at 30 days per API call.
func (c *Client) FetchConvertSince(ctx context.Context, since time.Time) ([]trade.Trade, error) {
	end := time.Now()
	if since.IsZero() {
		since = end.AddDate(0, 0, -180)
	}

	var out []trade.Trade
	const window = 30 * 24 * time.Hour
	for cursor := since; cursor.Before(end); cursor = cursor.Add(window) {
		windowEnd := cursor.Add(window)
		if windowEnd.After(end) {
			windowEnd = end
		}
		res, err := c.api.NewConvertTradeHistoryService().
			StartTime(cursor.UnixMilli()).
			EndTime(windowEnd.UnixMilli()).
			Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("convert history [%s..%s]: %w",
				cursor.Format(time.RFC3339), windowEnd.Format(time.RFC3339), err)
		}
		for _, item := range res.List {
			if item.OrderStatus != "SUCCESS" {
				continue
			}
			tr, ok, err := convertItemToTrade(item, c.cfg.QuoteAsset)
			if err != nil {
				return nil, fmt.Errorf("convert order %d: %w", item.OrderId, err)
			}
			if !ok {
				continue
			}
			out = append(out, tr)
		}
	}
	return out, nil
}

// convertItemToTrade maps a ConvertTradeHistoryItem into a domain Trade.
//
// Returns ok=false (without error) when the convert pair has no leg in the
// configured quote currency — those orders are skipped.
func convertItemToTrade(item bn.ConvertTradeHistoryItem, quote shared.Symbol) (trade.Trade, bool, error) {
	fromSym, err := shared.NewSymbol(item.FromAsset)
	if err != nil {
		return trade.Trade{}, false, err
	}
	toSym, err := shared.NewSymbol(item.ToAsset)
	if err != nil {
		return trade.Trade{}, false, err
	}

	fromAmt, err := decimal.NewFromString(item.FromAmount)
	if err != nil {
		return trade.Trade{}, false, fmt.Errorf("from amount: %w", err)
	}
	toAmt, err := decimal.NewFromString(item.ToAmount)
	if err != nil {
		return trade.Trade{}, false, fmt.Errorf("to amount: %w", err)
	}
	if fromAmt.IsZero() || toAmt.IsZero() {
		return trade.Trade{}, false, nil
	}

	var (
		side     trade.Side
		base     shared.Symbol
		baseQty  decimal.Decimal
		unitCost decimal.Decimal
	)
	switch {
	case fromSym.Equals(quote):
		// Spent quote → received base. BUY base.
		side = trade.SideBuy
		base = toSym
		baseQty = toAmt
		unitCost = fromAmt.Div(toAmt)
	case toSym.Equals(quote):
		// Spent base → received quote. SELL base.
		side = trade.SideSell
		base = fromSym
		baseQty = fromAmt
		unitCost = toAmt.Div(fromAmt)
	default:
		// Neither side is the configured quote currency.
		return trade.Trade{}, false, nil
	}

	qty, err := shared.NewQuantity(baseQty)
	if err != nil {
		return trade.Trade{}, false, err
	}
	priceMoney, err := shared.NewMoney(unitCost, quote)
	if err != nil {
		return trade.Trade{}, false, err
	}
	feeMoney, err := shared.NewMoney(decimal.Zero, quote)
	if err != nil {
		return trade.Trade{}, false, err
	}

	tr, err := trade.New(trade.Params{
		ID:         "convert-" + strconv.FormatInt(item.OrderId, 10),
		Asset:      base,
		Quote:      quote,
		Side:       side,
		Source:     shared.SourceConvert,
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
