// Package commands holds write-side use cases.
package commands

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/binancetracker/binancetracker/internal/application/ports"
	"github.com/binancetracker/binancetracker/internal/domain/acquisition"
	"github.com/binancetracker/binancetracker/internal/domain/asset"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/binancetracker/binancetracker/internal/domain/trade"
)

// totalSources is the number of distinct sources Sync attempts. Used to
// decide whether a run was a total failure (every source errored) or only a
// partial one.
const totalSources = 5

// SyncBinanceTrades pulls all transactional events from the configured exchange
// importer (spot trades, Convert orders, fiat buys, crypto deposits, Earn
// rewards) and persists them. Each source has its own watermark so that
// failures in one source don't reset progress in another.
type SyncBinanceTrades struct {
	importer     ports.ExchangeImporter
	trades       ports.TradeRepository
	acquisitions ports.AcquisitionRepository
	assets       ports.AssetRepository
	logger       ports.Logger
}

// NewSyncBinanceTrades wires the use case dependencies.
func NewSyncBinanceTrades(
	importer ports.ExchangeImporter,
	trades ports.TradeRepository,
	acquisitions ports.AcquisitionRepository,
	assets ports.AssetRepository,
	logger ports.Logger,
) *SyncBinanceTrades {
	return &SyncBinanceTrades{
		importer:     importer,
		trades:       trades,
		acquisitions: acquisitions,
		assets:       assets,
		logger:       logger,
	}
}

// SyncResult summarises a sync run, broken down by source so the UI can
// surface a useful "imported X new spot trades, Y new converts, …" message.
type SyncResult struct {
	Imported       int            // total rows persisted across all sources
	BySource       map[string]int // count per source
	PartialFailure bool           // true if at least one source errored
	Errors         []string       // human-readable error messages, one per failing source
}

// Execute runs the sync across all sources. The command tries every source
// even if some fail; failures are reported in the result rather than aborting
// the run, because partial success is still useful (e.g. spot worked but
// deposits hit a permission issue).
func (uc *SyncBinanceTrades) Execute(ctx context.Context) (SyncResult, error) {
	res := SyncResult{BySource: map[string]int{}}

	// 1. Spot trades
	uc.runTradeSource(ctx, &res, shared.SourceSpot, "spot trades", uc.importer.FetchTradesSince)
	// 2. Convert
	uc.runTradeSource(ctx, &res, shared.SourceConvert, "convert", uc.importer.FetchConvertSince)
	// 3. Fiat buys (Buy Crypto)
	uc.runTradeSource(ctx, &res, shared.SourceFiatBuy, "fiat buys", uc.importer.FetchFiatBuysSince)

	// 4. Crypto deposits
	uc.runAcquisitionSource(ctx, &res, shared.SourceDeposit, "deposits", uc.importer.FetchDepositsSince)
	// 5. Earn rewards
	uc.runAcquisitionSource(ctx, &res, shared.SourceEarnReward, "earn rewards", uc.importer.FetchEarnRewardsSince)

	// Bubble up an error only if EVERY source failed; otherwise the partial
	// result is the truthful answer and the UI can surface per-source warnings.
	if len(res.Errors) == totalSources {
		return res, errors.New(res.Errors[0])
	}
	return res, nil
}

// runTradeSource fetches and persists trades for one source, recording the
// outcome in the running SyncResult. It never returns errors directly — those
// go into result.Errors so the orchestrator can keep going.
func (uc *SyncBinanceTrades) runTradeSource(
	ctx context.Context,
	res *SyncResult,
	source shared.Source,
	label string,
	fetch func(ctx context.Context, since time.Time) ([]trade.Trade, error),
) {
	since, err := uc.tradeWatermark(ctx, source)
	if err != nil {
		uc.recordFailure(res, label, fmt.Errorf("watermark: %w", err))
		return
	}
	uc.logger.Info("syncing source", "source", source, "since", since)

	fetched, err := fetch(ctx, since)
	if err != nil {
		uc.recordFailure(res, label, err)
		return
	}
	if len(fetched) == 0 {
		res.BySource[string(source)] = 0
		return
	}
	if err := uc.trades.SaveBatch(ctx, fetched); err != nil {
		uc.recordFailure(res, label, fmt.Errorf("persist: %w", err))
		return
	}
	uc.upsertAssetsFromTrades(ctx, fetched)

	res.BySource[string(source)] = len(fetched)
	res.Imported += len(fetched)
}

func (uc *SyncBinanceTrades) runAcquisitionSource(
	ctx context.Context,
	res *SyncResult,
	source shared.Source,
	label string,
	fetch func(ctx context.Context, since time.Time) ([]acquisition.Acquisition, error),
) {
	since, err := uc.acquisitions.LatestAcquiredAt(ctx, source)
	if err != nil {
		uc.recordFailure(res, label, fmt.Errorf("watermark: %w", err))
		return
	}
	uc.logger.Info("syncing source", "source", source, "since", since)

	fetched, err := fetch(ctx, since)
	if err != nil {
		uc.recordFailure(res, label, err)
		return
	}
	if len(fetched) == 0 {
		res.BySource[string(source)] = 0
		return
	}
	if err := uc.acquisitions.SaveBatch(ctx, fetched); err != nil {
		uc.recordFailure(res, label, fmt.Errorf("persist: %w", err))
		return
	}
	uc.upsertAssetsFromAcquisitions(ctx, fetched)

	res.BySource[string(source)] = len(fetched)
	res.Imported += len(fetched)
}

func (uc *SyncBinanceTrades) recordFailure(res *SyncResult, label string, err error) {
	uc.logger.Warn("source sync failed", "source", label, "err", err)
	res.PartialFailure = true
	res.Errors = append(res.Errors, fmt.Sprintf("%s: %s", label, err.Error()))
}

// tradeWatermark returns the most recent ExecutedAt for the given source.
// We rely on a method that's available on the postgres TradeRepo but not on
// the abstract port — fall back to the global watermark if the repo doesn't
// expose per-source granularity (e.g. in tests with a mock).
func (uc *SyncBinanceTrades) tradeWatermark(ctx context.Context, source shared.Source) (time.Time, error) {
	if r, ok := uc.trades.(interface {
		LatestExecutedAtBySource(context.Context, shared.Source) (time.Time, error)
	}); ok {
		return r.LatestExecutedAtBySource(ctx, source)
	}
	return uc.trades.LatestExecutedAt(ctx)
}

func (uc *SyncBinanceTrades) upsertAssetsFromTrades(ctx context.Context, trades []trade.Trade) {
	seen := make(map[string]struct{}, len(trades))
	for _, t := range trades {
		sym := t.Asset()
		if _, ok := seen[sym.String()]; ok {
			continue
		}
		seen[sym.String()] = struct{}{}
		a, err := asset.New(sym, sym.String())
		if err != nil {
			continue
		}
		if err := uc.assets.Upsert(ctx, a); err != nil {
			uc.logger.Warn("upsert asset failed", "symbol", sym.String(), "err", err)
		}
	}
}

func (uc *SyncBinanceTrades) upsertAssetsFromAcquisitions(ctx context.Context, items []acquisition.Acquisition) {
	seen := make(map[string]struct{}, len(items))
	for _, it := range items {
		sym := it.Asset()
		if _, ok := seen[sym.String()]; ok {
			continue
		}
		seen[sym.String()] = struct{}{}
		a, err := asset.New(sym, sym.String())
		if err != nil {
			continue
		}
		if err := uc.assets.Upsert(ctx, a); err != nil {
			uc.logger.Warn("upsert asset failed", "symbol", sym.String(), "err", err)
		}
	}
}
