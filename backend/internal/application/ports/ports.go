// Package ports declares the interfaces (driven side) that the application
// layer depends on. Concrete implementations live in `internal/infrastructure`
// and are wired in at the composition root (`cmd/api/main.go`).
//
// This is the cornerstone of the hexagonal architecture: the application
// layer never imports infrastructure, it only depends on these abstractions.
package ports

import (
	"context"
	"time"

	"github.com/binancetracker/binancetracker/internal/domain/acquisition"
	"github.com/binancetracker/binancetracker/internal/domain/asset"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/binancetracker/binancetracker/internal/domain/trade"
	"github.com/shopspring/decimal"
)

// TradeRepository persists and retrieves trades.
type TradeRepository interface {
	// Save inserts a trade if its ID is unknown, otherwise it is a no-op.
	// The implementation MUST be idempotent so that repeated syncs don't
	// duplicate rows.
	Save(ctx context.Context, t trade.Trade) error
	// SaveBatch persists many trades atomically.
	SaveBatch(ctx context.Context, trades []trade.Trade) error
	// ListAll returns all known trades, oldest first.
	ListAll(ctx context.Context) ([]trade.Trade, error)
	// ListByAsset returns trades for one asset, oldest first.
	ListByAsset(ctx context.Context, asset shared.Symbol) ([]trade.Trade, error)
	// LatestExecutedAt returns the most recent ExecutedAt across all trades,
	// or the zero time if the table is empty. Used for incremental sync.
	LatestExecutedAt(ctx context.Context) (time.Time, error)
}

// AcquisitionRepository persists and retrieves non-trade acquisitions
// (crypto deposits, Earn rewards, manual funds). Cost basis is stored as
// imputed unit cost at the moment the asset hit the account.
type AcquisitionRepository interface {
	SaveBatch(ctx context.Context, items []acquisition.Acquisition) error
	ListAll(ctx context.Context) ([]acquisition.Acquisition, error)
	ListByAsset(ctx context.Context, asset shared.Symbol) ([]acquisition.Acquisition, error)
	// LatestAcquiredAt returns the most recent AcquiredAt for the given source,
	// or zero time if no rows exist. Used for incremental sync watermarks.
	LatestAcquiredAt(ctx context.Context, source shared.Source) (time.Time, error)
	Get(ctx context.Context, id string) (acquisition.Acquisition, error)
	Delete(ctx context.Context, id string) error
}

// AssetRepository persists known assets (symbols traded at least once).
type AssetRepository interface {
	Upsert(ctx context.Context, a asset.Asset) error
	List(ctx context.Context) ([]asset.Asset, error)
}

// PriceRepository caches the latest known price per asset symbol.
type PriceRepository interface {
	Upsert(ctx context.Context, symbol shared.Symbol, price shared.Money, fetchedAt time.Time) error
	Latest(ctx context.Context, symbol shared.Symbol) (shared.Money, time.Time, error)
	LatestMany(ctx context.Context, symbols []shared.Symbol) (map[shared.Symbol]shared.Money, error)
}

// SettingsRepository stores user-editable settings (encrypted at rest where
// appropriate, e.g. for API secrets).
type SettingsRepository interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

// ExchangeImporter fetches trades and acquisitions from a third-party
// exchange. binancetracker only ships a Binance implementation but the
// interface is generic. Each method handles one logical source so callers
// can run them independently and persist their watermarks separately.
type ExchangeImporter interface {
	// FetchTradesSince returns spot trades executed strictly after `since`.
	FetchTradesSince(ctx context.Context, since time.Time) ([]trade.Trade, error)
	// FetchConvertSince returns Binance Convert trades after `since`.
	FetchConvertSince(ctx context.Context, since time.Time) ([]trade.Trade, error)
	// FetchFiatBuysSince returns Buy Crypto (card/SEPA) purchases after `since`.
	FetchFiatBuysSince(ctx context.Context, since time.Time) ([]trade.Trade, error)
	// FetchDepositsSince returns crypto deposits after `since`. Cost basis is
	// imputed from the spot price at the moment of the deposit.
	FetchDepositsSince(ctx context.Context, since time.Time) ([]acquisition.Acquisition, error)
	// FetchEarnRewardsSince returns Simple Earn / staking rewards after `since`.
	// Cost basis is imputed from the spot price at the moment of the reward.
	FetchEarnRewardsSince(ctx context.Context, since time.Time) ([]acquisition.Acquisition, error)
}

// PriceFeed returns the latest market prices for assets, denominated in the
// configured quote currency.
type PriceFeed interface {
	LatestPrice(ctx context.Context, asset shared.Symbol) (shared.Money, error)
	LatestPrices(ctx context.Context, assets []shared.Symbol) (map[shared.Symbol]shared.Money, error)
}

// FxRateProvider returns the exchange rate between two currencies (e.g.
// USDT→EUR). Implementations may cache results — callers don't need to.
//
// The contract: Rate(from, to) returns the multiplier such that
//
//	amount_in_to = amount_in_from * Rate(from, to)
//
// Returns 1 when from == to. Returns an error if neither leg can be priced
// (e.g. an obscure altcoin with no spot pair against the FX hub).
type FxRateProvider interface {
	Rate(ctx context.Context, from, to shared.Symbol) (decimal.Decimal, error)
}

// HistoricalPriceFeed resolves the spot price of an asset at a specific past
// moment, denominated in the given quote currency. Used to backfill the cost
// basis of manual fund entries when the user knows when they got the coins
// but not the price.
type HistoricalPriceFeed interface {
	PriceAt(ctx context.Context, asset, quote shared.Symbol, at time.Time) (shared.Money, error)
}

// Logger is a tiny structured logger interface so that application code does
// not depend on a specific implementation. Adapters provide concrete loggers.
type Logger interface {
	Info(msg string, kv ...any)
	Warn(msg string, kv ...any)
	Error(msg string, kv ...any)
}
