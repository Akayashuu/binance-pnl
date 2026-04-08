// Package binance adapts the Binance Spot API to the binancetracker
// application ports (ExchangeImporter and PriceFeed).
//
// Binance enforces a per-symbol model for personal trade history: you must
// pass a symbol like BTCUSDT to GetMyTrades. We therefore iterate over a
// configured list of base assets and pair each one with the configured quote
// currency.
package binance

import (
	"context"
	"fmt"
	"strconv"
	"time"

	bn "github.com/adshao/go-binance/v2"
	"github.com/binancetracker/binancetracker/internal/application/ports"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/binancetracker/binancetracker/internal/domain/trade"
	"github.com/shopspring/decimal"
)

// Compile-time port assertions.
var (
	_ ports.ExchangeImporter = (*Client)(nil)
	_ ports.PriceFeed        = (*Client)(nil)
)

// Config configures a Binance client.
type Config struct {
	APIKey       string
	APISecret    string
	QuoteAsset   shared.Symbol // e.g. USDT
	TrackedBases []shared.Symbol
}

// Client wraps the go-binance SDK.
type Client struct {
	api *bn.Client
	cfg Config
}

// New constructs a Client.
func New(cfg Config) *Client {
	return &Client{
		api: bn.NewClient(cfg.APIKey, cfg.APISecret),
		cfg: cfg,
	}
}

// FetchTradesSince walks every tracked pair and pulls trades after `since`.
func (c *Client) FetchTradesSince(ctx context.Context, since time.Time) ([]trade.Trade, error) {
	var out []trade.Trade
	for _, base := range c.cfg.TrackedBases {
		pair := base.String() + c.cfg.QuoteAsset.String()
		svc := c.api.NewListTradesService().Symbol(pair)
		if !since.IsZero() {
			svc = svc.StartTime(since.UnixMilli() + 1)
		}
		res, err := svc.Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("list trades %s: %w", pair, err)
		}
		for _, t := range res {
			tr, err := convertTrade(t, base, c.cfg.QuoteAsset)
			if err != nil {
				return nil, fmt.Errorf("convert trade %d: %w", t.ID, err)
			}
			out = append(out, tr)
		}
	}
	return out, nil
}

// LatestPrice fetches the spot ticker for one base asset.
func (c *Client) LatestPrice(ctx context.Context, asset shared.Symbol) (shared.Money, error) {
	pair := asset.String() + c.cfg.QuoteAsset.String()
	res, err := c.api.NewListPricesService().Symbol(pair).Do(ctx)
	if err != nil {
		return shared.Money{}, err
	}
	if len(res) == 0 {
		return shared.Money{}, fmt.Errorf("no price for %s", pair)
	}
	amount, err := decimal.NewFromString(res[0].Price)
	if err != nil {
		return shared.Money{}, err
	}
	return shared.NewMoney(amount, c.cfg.QuoteAsset)
}

// LatestPrices fetches all spot tickers in one call and filters to the
// requested set. This avoids N HTTP round-trips when refreshing many assets.
func (c *Client) LatestPrices(ctx context.Context, assets []shared.Symbol) (map[shared.Symbol]shared.Money, error) {
	if len(assets) == 0 {
		return map[shared.Symbol]shared.Money{}, nil
	}

	res, err := c.api.NewListPricesService().Do(ctx)
	if err != nil {
		return nil, err
	}

	want := make(map[string]shared.Symbol, len(assets))
	for _, a := range assets {
		want[a.String()+c.cfg.QuoteAsset.String()] = a
	}

	out := make(map[shared.Symbol]shared.Money, len(assets))
	for _, p := range res {
		base, ok := want[p.Symbol]
		if !ok {
			continue
		}
		amount, err := decimal.NewFromString(p.Price)
		if err != nil {
			continue
		}
		m, err := shared.NewMoney(amount, c.cfg.QuoteAsset)
		if err != nil {
			continue
		}
		out[base] = m
	}
	return out, nil
}

// convertTrade maps a Binance trade DTO into the domain Trade.
func convertTrade(t *bn.TradeV3, base, quote shared.Symbol) (trade.Trade, error) {
	side := trade.SideSell
	if t.IsBuyer {
		side = trade.SideBuy
	}

	qtyDec, err := decimal.NewFromString(t.Quantity)
	if err != nil {
		return trade.Trade{}, err
	}
	qty, err := shared.NewQuantity(qtyDec)
	if err != nil {
		return trade.Trade{}, err
	}

	priceDec, err := decimal.NewFromString(t.Price)
	if err != nil {
		return trade.Trade{}, err
	}
	price, err := shared.NewMoney(priceDec, quote)
	if err != nil {
		return trade.Trade{}, err
	}

	feeDec, err := decimal.NewFromString(t.Commission)
	if err != nil {
		return trade.Trade{}, err
	}
	feeSym, err := shared.NewSymbol(t.CommissionAsset)
	if err != nil {
		return trade.Trade{}, err
	}
	fee, err := shared.NewMoney(feeDec, feeSym)
	if err != nil {
		return trade.Trade{}, err
	}

	return trade.New(trade.Params{
		// Keep raw numeric ID for spot trades to remain idempotent with rows
		// inserted before the source column existed.
		ID:         strconv.FormatInt(t.ID, 10),
		Asset:      base,
		Quote:      quote,
		Side:       side,
		Source:     shared.SourceSpot,
		Quantity:   qty,
		Price:      price,
		Fee:        fee,
		ExecutedAt: time.UnixMilli(t.Time),
	})
}
