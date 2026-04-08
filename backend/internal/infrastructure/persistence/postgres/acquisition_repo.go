package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/binancetracker/binancetracker/internal/application/ports"
	"github.com/binancetracker/binancetracker/internal/domain/acquisition"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Compile-time assertion that AcquisitionRepo implements the port.
var _ ports.AcquisitionRepository = (*AcquisitionRepo)(nil)

// AcquisitionRepo persists Acquisition aggregates in PostgreSQL.
type AcquisitionRepo struct {
	pool *pgxpool.Pool
}

// NewAcquisitionRepo wires the repository.
func NewAcquisitionRepo(pool *pgxpool.Pool) *AcquisitionRepo {
	return &AcquisitionRepo{pool: pool}
}

// SaveBatch inserts many acquisitions in a single transaction. Conflicts on
// primary key are silently ignored, making the operation idempotent.
func (r *AcquisitionRepo) SaveBatch(ctx context.Context, items []acquisition.Acquisition) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Ensure asset rows exist for FK constraint.
	for _, a := range items {
		if _, err := tx.Exec(ctx,
			`INSERT INTO assets (symbol, name) VALUES ($1, $1)
			 ON CONFLICT (symbol) DO NOTHING`,
			a.Asset().String(),
		); err != nil {
			return fmt.Errorf("upsert asset: %w", err)
		}
	}

	for _, a := range items {
		_, err := tx.Exec(ctx,
			`INSERT INTO acquisitions
			 (id, asset, quote, source, quantity, unit_cost, acquired_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7)
			 ON CONFLICT (id) DO NOTHING`,
			a.ID(),
			a.Asset().String(),
			a.Quote().String(),
			string(a.Source()),
			a.Quantity().Decimal().String(),
			a.UnitCost().Amount().String(),
			a.AcquiredAt().UTC(),
		)
		if err != nil {
			return fmt.Errorf("insert acquisition %s: %w", a.ID(), err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// ListAll returns all acquisitions, oldest first.
func (r *AcquisitionRepo) ListAll(ctx context.Context) ([]acquisition.Acquisition, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, asset, quote, source, quantity, unit_cost, acquired_at
		 FROM acquisitions ORDER BY acquired_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAcquisitions(rows)
}

// ListByAsset returns acquisitions for a single asset, oldest first.
func (r *AcquisitionRepo) ListByAsset(ctx context.Context, asset shared.Symbol) ([]acquisition.Acquisition, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, asset, quote, source, quantity, unit_cost, acquired_at
		 FROM acquisitions WHERE asset = $1 ORDER BY acquired_at ASC`,
		asset.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAcquisitions(rows)
}

// Get fetches a single acquisition by id. Returns pgx.ErrNoRows when missing.
func (r *AcquisitionRepo) Get(ctx context.Context, id string) (acquisition.Acquisition, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, asset, quote, source, quantity, unit_cost, acquired_at
		 FROM acquisitions WHERE id = $1`,
		id)
	if err != nil {
		return acquisition.Acquisition{}, err
	}
	defer rows.Close()
	out, err := scanAcquisitions(rows)
	if err != nil {
		return acquisition.Acquisition{}, err
	}
	if len(out) == 0 {
		return acquisition.Acquisition{}, pgx.ErrNoRows
	}
	return out[0], nil
}

// Delete removes an acquisition by id. No error if it didn't exist.
func (r *AcquisitionRepo) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM acquisitions WHERE id = $1`, id)
	return err
}

// LatestAcquiredAt returns the most recent AcquiredAt for the given source.
func (r *AcquisitionRepo) LatestAcquiredAt(ctx context.Context, source shared.Source) (time.Time, error) {
	var ts *time.Time
	err := r.pool.QueryRow(ctx,
		`SELECT MAX(acquired_at) FROM acquisitions WHERE source = $1`,
		string(source)).Scan(&ts)
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

func scanAcquisitions(rows pgx.Rows) ([]acquisition.Acquisition, error) {
	var out []acquisition.Acquisition
	for rows.Next() {
		var (
			id, asset, quote, source string
			quantity, unitCost       decimal.Decimal
			acquiredAt               time.Time
		)
		if err := rows.Scan(&id, &asset, &quote, &source, &quantity, &unitCost, &acquiredAt); err != nil {
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
		src, err := shared.ParseSource(source)
		if err != nil {
			return nil, fmt.Errorf("acquisition %s: %w", id, err)
		}
		qty, err := shared.NewQuantity(quantity)
		if err != nil {
			return nil, err
		}
		unitCostMoney, err := shared.NewMoney(unitCost, quoteSym)
		if err != nil {
			return nil, err
		}
		a, err := acquisition.New(acquisition.Params{
			ID:         id,
			Asset:      assetSym,
			Quote:      quoteSym,
			Source:     src,
			Quantity:   qty,
			UnitCost:   unitCostMoney,
			AcquiredAt: acquiredAt,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
