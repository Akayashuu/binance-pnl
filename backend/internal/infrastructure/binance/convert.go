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

	primaryQuote := c.cfg.QuoteAsset
	execAt := time.UnixMilli(item.CreateTime)
	orderID := strconv.FormatInt(item.OrderId, 10)

	// Check if either leg matches any accepted quote currency.
	matchedQuote, fromIsQuote, toIsQuote := c.matchAcceptedQuote(fromSym, toSym)

	switch {
	case fromIsQuote:
		// Spent an accepted quote → received base. Single BUY.
		feeMoney, err := shared.NewMoney(decimal.Zero, matchedQuote)
		if err != nil {
			return nil, err
		}
		tr, err := buildConvertTrade(
			"convert-"+orderID, toSym, matchedQuote, trade.SideBuy,
			toAmt, fromAmt.Div(toAmt), feeMoney, execAt,
		)
		if err != nil {
			return nil, err
		}
		return []trade.Trade{tr}, nil

	case toIsQuote:
		// Spent base → received an accepted quote. Single SELL.
		feeMoney, err := shared.NewMoney(decimal.Zero, matchedQuote)
		if err != nil {
			return nil, err
		}
		tr, err := buildConvertTrade(
			"convert-"+orderID, fromSym, matchedQuote, trade.SideSell,
			fromAmt, toAmt.Div(fromAmt), feeMoney, execAt,
		)
		if err != nil {
			return nil, err
		}
		return []trade.Trade{tr}, nil

	default:
		// Cross-convert: neither leg is an accepted quote currency.
		// Look up historical prices to value both legs in the primary quote.
		feeMoney, err := shared.NewMoney(decimal.Zero, primaryQuote)
		if err != nil {
			return nil, err
		}
		fromPrice, err := c.PriceAt(ctx, fromSym, primaryQuote, execAt)
		if err != nil {
			return nil, fmt.Errorf("price lookup %s: %w", fromSym, err)
		}
		toPrice, err := c.PriceAt(ctx, toSym, primaryQuote, execAt)
		if err != nil {
			return nil, fmt.Errorf("price lookup %s: %w", toSym, err)
		}

		sellTr, err := buildConvertTrade(
			"convert-"+orderID+"-sell", fromSym, primaryQuote, trade.SideSell,
			fromAmt, fromPrice.Amount(), feeMoney, execAt,
		)
		if err != nil {
			return nil, err
		}
		buyTr, err := buildConvertTrade(
			"convert-"+orderID+"-buy", toSym, primaryQuote, trade.SideBuy,
			toAmt, toPrice.Amount(), feeMoney, execAt,
		)
		if err != nil {
			return nil, err
		}
		return []trade.Trade{sellTr, buyTr}, nil
	}
}

// matchAcceptedQuote checks whether either symbol matches one of the accepted
// quote currencies. Returns the matched quote and which side matched.
func (c *Client) matchAcceptedQuote(from, to shared.Symbol) (matched shared.Symbol, fromIsQuote, toIsQuote bool) {
	for _, q := range c.cfg.AcceptedQuotes {
		if from.Equals(q) {
			return q, true, false
		}
		if to.Equals(q) {
			return q, false, true
		}
	}
	return "", false, false
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
