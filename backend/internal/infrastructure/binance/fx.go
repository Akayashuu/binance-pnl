package binance

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/binancetracker/binancetracker/internal/application/ports"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/shopspring/decimal"
)

// fxHub is the asset routed through to bridge any two currencies that don't
// trade directly against each other on Binance. USDT is by far the most
// liquid pair on Binance, so it's the universal bridge.
const fxHub = "USDT"

// fxTTL is how long a cached rate stays fresh. 60 seconds keeps the dashboard
// snappy without overstating accuracy: an FX rate moving more than a fraction
// of a percent in a minute is unusual and Binance ticker prices update at
// roughly that cadence.
const fxTTL = 60 * time.Second

// FxClient is a tiny TTL-cached FX rate provider backed by Binance's
// /api/v3/ticker/price endpoint. It implements ports.FxRateProvider.
//
// It does NOT need API credentials — the ticker endpoint is public — so we
// can build it independently of the authenticated Client. This matters
// because at startup we typically don't have valid credentials yet.
type FxClient struct {
	api   bnPriceClient
	mu    sync.Mutex
	cache map[string]fxEntry
}

// bnPriceClient is the minimal subset of the SDK Client we depend on, to make
// the FxClient testable in isolation. The real Client satisfies it.
type bnPriceClient interface {
	priceFor(ctx context.Context, symbol string) (decimal.Decimal, error)
}

type fxEntry struct {
	rate decimal.Decimal
	at   time.Time
}

// Compile-time assertion.
var _ ports.FxRateProvider = (*FxClient)(nil)

// NewFxClient builds an FX client backed by an unauthenticated SDK Client.
func NewFxClient() *FxClient {
	return &FxClient{
		api:   newBinancePriceClient(),
		cache: map[string]fxEntry{},
	}
}

// Rate returns the multiplier such that `amount_to = amount_from * Rate(from, to)`.
//
// Routing:
//
//   - same currency → 1
//   - direct pair fromTO exists → ticker(fromTO).price
//   - inverse pair toFROM exists → 1 / ticker(toFROM).price
//   - otherwise: cross via USDT → Rate(from, USDT) * Rate(USDT, to)
func (f *FxClient) Rate(ctx context.Context, from, to shared.Symbol) (decimal.Decimal, error) {
	if from.Equals(to) {
		return decimal.NewFromInt(1), nil
	}

	cacheKey := from.String() + "/" + to.String()
	if r, ok := f.lookup(cacheKey); ok {
		return r, nil
	}

	rate, err := f.fetch(ctx, from, to)
	if err != nil {
		return decimal.Zero, err
	}
	f.store(cacheKey, rate)
	return rate, nil
}

func (f *FxClient) lookup(key string) (decimal.Decimal, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.cache[key]
	if !ok {
		return decimal.Zero, false
	}
	if time.Since(e.at) > fxTTL {
		delete(f.cache, key)
		return decimal.Zero, false
	}
	return e.rate, true
}

func (f *FxClient) store(key string, rate decimal.Decimal) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cache[key] = fxEntry{rate: rate, at: time.Now()}
}

func (f *FxClient) fetch(ctx context.Context, from, to shared.Symbol) (decimal.Decimal, error) {
	// Try direct pair first.
	if r, err := f.api.priceFor(ctx, from.String()+to.String()); err == nil {
		return r, nil
	}
	// Try inverse pair.
	if r, err := f.api.priceFor(ctx, to.String()+from.String()); err == nil {
		if r.IsZero() {
			return decimal.Zero, fmt.Errorf("inverse rate %s%s is zero", to, from)
		}
		return decimal.NewFromInt(1).Div(r), nil
	}
	// Cross via USDT.
	if from.String() == fxHub || to.String() == fxHub {
		return decimal.Zero, fmt.Errorf("no FX route from %s to %s", from, to)
	}
	hub, err := shared.NewSymbol(fxHub)
	if err != nil {
		return decimal.Zero, err
	}
	r1, err := f.Rate(ctx, from, hub)
	if err != nil {
		return decimal.Zero, fmt.Errorf("from %s to USDT: %w", from, err)
	}
	r2, err := f.Rate(ctx, hub, to)
	if err != nil {
		return decimal.Zero, fmt.Errorf("from USDT to %s: %w", to, err)
	}
	return r1.Mul(r2), nil
}
