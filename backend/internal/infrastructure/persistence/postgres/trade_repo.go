package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/binancetracker/binancetracker/internal/application/ports"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/binancetracker/binancetracker/internal/domain/trade"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Compile-time assertion that TradeRepo implements the port.
var _ ports.TradeRepository = (*TradeRepo)(nil)

// TradeRepo persists Trade aggregates in PostgreSQL.
type TradeRepo struct {
	pool *pgxpool.Pool
}

// NewTradeRepo wires the repository.
func NewTradeRepo(pool *pgxpool.Pool) *TradeRepo {
	return &TradeRepo{pool: pool}
}

// Save inserts a single trade idempotently.
func (r *TradeRepo) Save(ctx context.Context, t trade.Trade) error {
	return r.SaveBatch(ctx, []trade.Trade{t})
}

// SaveBatch inserts many trades in a single transaction. Conflicts on
// primary key are silently ignored, making the operation idempotent.
func (r *TradeRepo) SaveBatch(ctx context.Context, trades []trade.Trade) error {
	if len(trades) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Ensure asset rows exist for FK constraint.
	for _, t := range trades {
		if _, err := tx.Exec(ctx,
			`INSERT INTO assets (symbol, name) VALUES ($1, $1)
			 ON CONFLICT (symbol) DO NOTHING`,
			t.Asset().String(),
		); err != nil {
			return fmt.Errorf("upsert asset: %w", err)
		}
	}

	for _, t := range trades {
		_, err := tx.Exec(ctx,
			`INSERT INTO trades
			 (id, asset, quote, side, source, quantity, price, fee, fee_asset, executed_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			 ON CONFLICT (id) DO NOTHING`,
			t.ID(),
			t.Asset().String(),
			t.Quote().String(),
			string(t.Side()),
			string(t.Source()),
			t.Quantity().Decimal().String(),
			t.Price().Amount().String(),
			t.Fee().Amount().String(),
			t.Fee().Currency().String(),
			t.ExecutedAt().UTC(),
		)
		if err != nil {
			return fmt.Errorf("insert trade %s: %w", t.ID(), err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// ListAll returns all trades, oldest first.
func (r *TradeRepo) ListAll(ctx context.Context) ([]trade.Trade, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, asset, quote, side, source, quantity, price, fee, fee_asset, executed_at
		 FROM trades ORDER BY executed_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTrades(rows)
}

// ListByAsset returns trades for a single asset, oldest first.
func (r *TradeRepo) ListByAsset(ctx context.Context, asset shared.Symbol) ([]trade.Trade, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, asset, quote, side, source, quantity, price, fee, fee_asset, executed_at
		 FROM trades WHERE asset = $1 ORDER BY executed_at ASC`,
		asset.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTrades(rows)
}

// LatestExecutedAt returns the most recent ExecutedAt or zero time if empty.
func (r *TradeRepo) LatestExecutedAt(ctx context.Context) (time.Time, error) {
	return r.LatestExecutedAtBySource(ctx, "")
}

// LatestExecutedAtBySource returns the most recent ExecutedAt for the given
// source. Pass an empty source to query across all sources.
func (r *TradeRepo) LatestExecutedAtBySource(ctx context.Context, source shared.Source) (time.Time, error) {
	var (
		ts  *time.Time
		err error
	)
	if source == "" {
		err = r.pool.QueryRow(ctx, `SELECT MAX(executed_at) FROM trades`).Scan(&ts)
	} else {
		err = r.pool.QueryRow(ctx,
			`SELECT MAX(executed_at) FROM trades WHERE source = $1`,
			string(source)).Scan(&ts)
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	if ts == nil {
		return time.Time{}, nil
	}
	return *ts, nil
}

func scanTrades(rows pgx.Rows) ([]trade.Trade, error) {
	var out []trade.Trade
	for rows.Next() {
		var (
			id, asset, quote, side, source, feeAsset string
			quantity, price, fee                     decimal.Decimal
			executedAt                               time.Time
		)
		if err := rows.Scan(&id, &asset, &quote, &side, &source, &quantity, &price, &fee, &feeAsset, &executedAt); err != nil {
			return nil, err
		}
		assetSym, err := shared.NewSymbol(asset)
		if err != nil {
			return nil, err
		}
		quoteSym, err := shared.NewSymbol(quote)
		if err != nil {
			return nil, err
		}
		feeSym, err := shared.NewSymbol(feeAsset)
		if err != nil {
			return nil, err
		}
		qty, err := shared.NewQuantity(quantity)
		if err != nil {
			return nil, err
		}
		priceMoney, err := shared.NewMoney(price, quoteSym)
		if err != nil {
			return nil, err
		}
		feeMoney, err := shared.NewMoney(fee, feeSym)
		if err != nil {
			return nil, err
		}
		src, err := shared.ParseSource(source)
		if err != nil {
			return nil, fmt.Errorf("trade %s: %w", id, err)
		}
		t, err := trade.New(trade.Params{
			ID:         id,
			Asset:      assetSym,
			Quote:      quoteSym,
			Side:       trade.Side(side),
			Source:     src,
			Quantity:   qty,
			Price:      priceMoney,
			Fee:        feeMoney,
			ExecutedAt: executedAt,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
