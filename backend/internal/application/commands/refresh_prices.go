package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/binancetracker/binancetracker/internal/application/ports"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
)

// RefreshPrices pulls the latest market price for every known asset from the
// PriceFeed and stores it in the price cache.
type RefreshPrices struct {
	feed   ports.PriceFeed
	assets ports.AssetRepository
	prices ports.PriceRepository
	logger ports.Logger
	now    func() time.Time
}

// NewRefreshPrices wires the use case.
func NewRefreshPrices(
	feed ports.PriceFeed,
	assets ports.AssetRepository,
	prices ports.PriceRepository,
	logger ports.Logger,
) *RefreshPrices {
	return &RefreshPrices{
		feed:   feed,
		assets: assets,
		prices: prices,
		logger: logger,
		now:    time.Now,
	}
}

// Execute fetches and stores prices. Returns the number of prices updated.
func (uc *RefreshPrices) Execute(ctx context.Context) (int, error) {
	known, err := uc.assets.List(ctx)
	if err != nil {
		return 0, fmt.Errorf("list assets: %w", err)
	}
	if len(known) == 0 {
		return 0, nil
	}

	symbols := make([]shared.Symbol, 0, len(known))
	for _, a := range known {
		symbols = append(symbols, a.Symbol())
	}

	prices, err := uc.feed.LatestPrices(ctx, symbols)
	if err != nil {
		return 0, fmt.Errorf("fetch prices: %w", err)
	}

	now := uc.now()
	updated := 0
	for sym, price := range prices {
		if err := uc.prices.Upsert(ctx, sym, price, now); err != nil {
			uc.logger.Warn("price upsert failed", "symbol", sym.String(), "err", err)
			continue
		}
		updated++
	}
	return updated, nil
}
