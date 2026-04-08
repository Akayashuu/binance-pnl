package commands_test

import (
	"context"
	"testing"
	"time"

	"github.com/binancetracker/binancetracker/internal/application/commands"
	"github.com/binancetracker/binancetracker/internal/domain/acquisition"
	"github.com/binancetracker/binancetracker/internal/domain/asset"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/binancetracker/binancetracker/internal/domain/trade"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test doubles ----------------------------------------------------------

type fakeImporter struct {
	spot []trade.Trade
}

func (f *fakeImporter) FetchTradesSince(_ context.Context, _ time.Time) ([]trade.Trade, error) {
	return f.spot, nil
}
func (f *fakeImporter) FetchConvertSince(_ context.Context, _ time.Time) ([]trade.Trade, error) {
	return nil, nil
}
func (f *fakeImporter) FetchFiatBuysSince(_ context.Context, _ time.Time) ([]trade.Trade, error) {
	return nil, nil
}
func (f *fakeImporter) FetchDepositsSince(_ context.Context, _ time.Time) ([]acquisition.Acquisition, error) {
	return nil, nil
}
func (f *fakeImporter) FetchEarnRewardsSince(_ context.Context, _ time.Time) ([]acquisition.Acquisition, error) {
	return nil, nil
}

type fakeTradeRepo struct {
	saved        []trade.Trade
	latestRet    time.Time
	saveBatchErr error
}

func (r *fakeTradeRepo) Save(_ context.Context, t trade.Trade) error {
	r.saved = append(r.saved, t)
	return nil
}
func (r *fakeTradeRepo) SaveBatch(_ context.Context, ts []trade.Trade) error {
	if r.saveBatchErr != nil {
		return r.saveBatchErr
	}
	r.saved = append(r.saved, ts...)
	return nil
}
func (r *fakeTradeRepo) ListAll(_ context.Context) ([]trade.Trade, error) {
	return r.saved, nil
}
func (r *fakeTradeRepo) ListByAsset(_ context.Context, _ shared.Symbol) ([]trade.Trade, error) {
	return r.saved, nil
}
func (r *fakeTradeRepo) LatestExecutedAt(_ context.Context) (time.Time, error) {
	return r.latestRet, nil
}

type fakeAcquisitionRepo struct {
	saved []acquisition.Acquisition
}

func (r *fakeAcquisitionRepo) SaveBatch(_ context.Context, items []acquisition.Acquisition) error {
	r.saved = append(r.saved, items...)
	return nil
}
func (r *fakeAcquisitionRepo) ListAll(_ context.Context) ([]acquisition.Acquisition, error) {
	return r.saved, nil
}
func (r *fakeAcquisitionRepo) ListByAsset(_ context.Context, _ shared.Symbol) ([]acquisition.Acquisition, error) {
	return r.saved, nil
}
func (r *fakeAcquisitionRepo) LatestAcquiredAt(_ context.Context, _ shared.Source) (time.Time, error) {
	return time.Time{}, nil
}
func (r *fakeAcquisitionRepo) Get(_ context.Context, _ string) (acquisition.Acquisition, error) {
	return acquisition.Acquisition{}, nil
}
func (r *fakeAcquisitionRepo) Delete(_ context.Context, _ string) error { return nil }

type fakeAssetRepo struct {
	upserted []asset.Asset
}

func (r *fakeAssetRepo) Upsert(_ context.Context, a asset.Asset) error {
	r.upserted = append(r.upserted, a)
	return nil
}
func (r *fakeAssetRepo) List(_ context.Context) ([]asset.Asset, error) { return r.upserted, nil }

type nopLogger struct{}

func (nopLogger) Info(string, ...any)  {}
func (nopLogger) Warn(string, ...any)  {}
func (nopLogger) Error(string, ...any) {}

// --- Helpers ---------------------------------------------------------------

func mkTrade(t *testing.T, id, sym string, when time.Time) trade.Trade {
	t.Helper()
	asset, _ := shared.NewSymbol(sym)
	usdt, _ := shared.NewSymbol("USDT")
	tr, err := trade.New(trade.Params{
		ID:         id,
		Asset:      asset,
		Quote:      usdt,
		Side:       trade.SideBuy,
		Source:     shared.SourceSpot,
		Quantity:   shared.MustQuantityFromString("1"),
		Price:      shared.MustMoneyFromString("100", "USDT"),
		Fee:        shared.MustMoneyFromString("0", "USDT"),
		ExecutedAt: when,
	})
	require.NoError(t, err)
	return tr
}

// --- Tests -----------------------------------------------------------------

func TestSyncBinanceTrades_Success(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)
	importer := &fakeImporter{spot: []trade.Trade{
		mkTrade(t, "1", "BTC", now),
		mkTrade(t, "2", "ETH", now.Add(time.Minute)),
		mkTrade(t, "3", "BTC", now.Add(2*time.Minute)),
	}}
	trades := &fakeTradeRepo{}
	acqs := &fakeAcquisitionRepo{}
	assets := &fakeAssetRepo{}

	uc := commands.NewSyncBinanceTrades(importer, trades, acqs, assets, nopLogger{})

	res, err := uc.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 3, res.Imported)
	assert.Equal(t, 3, res.BySource["spot"])
	assert.Len(t, trades.saved, 3)
	assert.Len(t, assets.upserted, 2) // BTC and ETH, dedup
}

func TestSyncBinanceTrades_NoNew(t *testing.T) {
	t.Parallel()
	importer := &fakeImporter{spot: nil}
	trades := &fakeTradeRepo{}
	acqs := &fakeAcquisitionRepo{}
	assets := &fakeAssetRepo{}

	uc := commands.NewSyncBinanceTrades(importer, trades, acqs, assets, nopLogger{})

	res, err := uc.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, res.Imported)
	assert.Empty(t, trades.saved)
}
