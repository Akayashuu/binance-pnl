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
	APIKey         string
	APISecret      string
	QuoteAsset     shared.Symbol   // primary quote for pricing (e.g. USDT)
	AcceptedQuotes []shared.Symbol // all quotes to fetch trades for (e.g. USDT, USDC)
	TrackedBases   []shared.Symbol
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

// FetchTradesSince walks every tracked pair across all accepted quote
// currencies and pulls trades after `since`.
//
// Binance has no "all my trades" endpoint — GetMyTrades requires a symbol —
// so we have to enumerate the bases ourselves. We start from the configured
// TrackedBases and augment them with every coin currently sitting in the
// user's account (free or locked). That way users don't need to maintain
// TRACKED_ASSETS by hand: any coin they hold gets picked up automatically.
func (c *Client) FetchTradesSince(ctx context.Context, since time.Time) ([]trade.Trade, error) {
	bases := c.resolveBases(ctx)

	var out []trade.Trade
	for _, base := range bases {
		for _, quote := range c.cfg.AcceptedQuotes {
			if base.Equals(quote) {
				continue
			}
			pair := base.String() + quote.String()
			svc := c.api.NewListTradesService().Symbol(pair)
			if !since.IsZero() {
				svc = svc.StartTime(since.UnixMilli() + 1)
			}
			res, err := svc.Do(ctx)
			if err != nil {
				// Pair may not exist on Binance (e.g. SOLUSDC) — skip gracefully.
				continue
			}
			for _, t := range res {
				tr, err := convertTrade(t, base, quote)
				if err != nil {
					return nil, fmt.Errorf("convert trade %d: %w", t.ID, err)
				}
				out = append(out, tr)
			}
		}
	}
	return out, nil
}

// resolveBases returns the set of base assets to fetch trades for. It always
// includes the configured TrackedBases and, when account access is available,
// every non-zero balance on the account. Failures to read balances are not
// fatal — we fall back to TrackedBases alone.
func (c *Client) resolveBases(ctx context.Context) []shared.Symbol {
	seen := make(map[shared.Symbol]struct{}, len(c.cfg.TrackedBases))
	out := make([]shared.Symbol, 0, len(c.cfg.TrackedBases))
	add := func(s shared.Symbol) {
		if s.IsZero() {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	for _, b := range c.cfg.TrackedBases {
		add(b)
	}

	acct, err := c.api.NewGetAccountService().Do(ctx)
	if err != nil {
		return out
	}
	for _, b := range acct.Balances {
		free, _ := decimal.NewFromString(b.Free)
		locked, _ := decimal.NewFromString(b.Locked)
		if free.Add(locked).IsZero() {
			continue
		}
		sym, err := shared.NewSymbol(b.Asset)
		if err != nil {
			continue
		}
		add(sym)
	}
	return out
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
