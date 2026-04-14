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

// FetchConvertSince walks Binance Convert history. For cross-converts (neither
// leg in the configured quote), a historical price lookup is used so that both
// sides are valued in the quote currency. Window is capped at 30 days per API call.
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
			trades, err := c.convertItemToTrades(ctx, item)
			if err != nil {
				return nil, fmt.Errorf("convert order %d: %w", item.OrderId, err)
			}
			out = append(out, trades...)
		}
	}
	return out, nil
}

// convertItemToTrades maps a ConvertTradeHistoryItem into one or two domain
// Trades.
//
// When one leg matches the configured quote currency, a single trade is
// returned (BUY or SELL). For cross-converts (neither leg is the quote), two
// synthetic trades are produced: a SELL of the source asset and a BUY of the
// destination asset, both priced via a historical kline lookup so the P&L
// stays consistent.
func (c *Client) convertItemToTrades(ctx context.Context, item bn.ConvertTradeHistoryItem) ([]trade.Trade, error) {
	fromSym, err := shared.NewSymbol(item.FromAsset)
	if err != nil {
		return nil, err
	}
	toSym, err := shared.NewSymbol(item.ToAsset)
	if err != nil {
		return nil, err
	}

	fromAmt, err := decimal.NewFromString(item.FromAmount)
	if err != nil {
		return nil, fmt.Errorf("from amount: %w", err)
	}
	toAmt, err := decimal.NewFromString(item.ToAmount)
	if err != nil {
		return nil, fmt.Errorf("to amount: %w", err)
	}
	if fromAmt.IsZero() || toAmt.IsZero() {
		return nil, nil
	}

	quote := c.cfg.QuoteAsset
	execAt := time.UnixMilli(item.CreateTime)
	orderID := strconv.FormatInt(item.OrderId, 10)
	feeMoney, err := shared.NewMoney(decimal.Zero, quote)
	if err != nil {
		return nil, err
	}

	switch {
	case fromSym.Equals(quote):
		// Spent quote → received base. Single BUY.
		tr, err := buildConvertTrade(
			"convert-"+orderID, toSym, quote, trade.SideBuy,
			toAmt, fromAmt.Div(toAmt), feeMoney, execAt,
		)
		if err != nil {
			return nil, err
		}
		return []trade.Trade{tr}, nil

	case toSym.Equals(quote):
		// Spent base → received quote. Single SELL.
		tr, err := buildConvertTrade(
			"convert-"+orderID, fromSym, quote, trade.SideSell,
			fromAmt, toAmt.Div(fromAmt), feeMoney, execAt,
		)
		if err != nil {
			return nil, err
		}
		return []trade.Trade{tr}, nil

	default:
		// Cross-convert: neither leg is the quote currency.
		// Look up historical prices to value both legs in the quote.
		fromPrice, err := c.PriceAt(ctx, fromSym, quote, execAt)
		if err != nil {
			return nil, fmt.Errorf("price lookup %s: %w", fromSym, err)
		}
		toPrice, err := c.PriceAt(ctx, toSym, quote, execAt)
		if err != nil {
			return nil, fmt.Errorf("price lookup %s: %w", toSym, err)
		}

		sellTr, err := buildConvertTrade(
			"convert-"+orderID+"-sell", fromSym, quote, trade.SideSell,
			fromAmt, fromPrice.Amount(), feeMoney, execAt,
		)
		if err != nil {
			return nil, err
		}
		buyTr, err := buildConvertTrade(
			"convert-"+orderID+"-buy", toSym, quote, trade.SideBuy,
			toAmt, toPrice.Amount(), feeMoney, execAt,
		)
		if err != nil {
			return nil, err
		}
		return []trade.Trade{sellTr, buyTr}, nil
	}
}

// buildConvertTrade creates a single convert trade with the given parameters.
func buildConvertTrade(
	id string,
	asset, quote shared.Symbol,
	side trade.Side,
	quantity, unitPrice decimal.Decimal,
	fee shared.Money,
	execAt time.Time,
) (trade.Trade, error) {
	qty, err := shared.NewQuantity(quantity)
	if err != nil {
		return trade.Trade{}, err
	}
	priceMoney, err := shared.NewMoney(unitPrice, quote)
	if err != nil {
		return trade.Trade{}, err
	}
	return trade.New(trade.Params{
		ID:         id,
		Asset:      asset,
		Quote:      quote,
		Side:       side,
		Source:     shared.SourceConvert,
		Quantity:   qty,
		Price:      priceMoney,
		Fee:        fee,
		ExecutedAt: execAt,
	})
}
