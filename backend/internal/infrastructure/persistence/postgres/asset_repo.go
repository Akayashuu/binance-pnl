package postgres

import (
	"context"

	"github.com/binancetracker/binancetracker/internal/application/ports"
	"github.com/binancetracker/binancetracker/internal/domain/asset"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ ports.AssetRepository = (*AssetRepo)(nil)

// AssetRepo persists Asset aggregates.
type AssetRepo struct {
	pool *pgxpool.Pool
}

// NewAssetRepo wires the repository.
func NewAssetRepo(pool *pgxpool.Pool) *AssetRepo { return &AssetRepo{pool: pool} }

// Upsert inserts or updates an asset by symbol.
func (r *AssetRepo) Upsert(ctx context.Context, a asset.Asset) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO assets (symbol, name) VALUES ($1, $2)
		 ON CONFLICT (symbol) DO UPDATE SET name = EXCLUDED.name`,
		a.Symbol().String(), a.Name())
	return err
}

// List returns all known assets.
func (r *AssetRepo) List(ctx context.Context) ([]asset.Asset, error) {
	rows, err := r.pool.Query(ctx, `SELECT symbol, name FROM assets ORDER BY symbol`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []asset.Asset
	for rows.Next() {
		var symbol, name string
		if err := rows.Scan(&symbol, &name); err != nil {
			return nil, err
		}
		sym, err := shared.NewSymbol(symbol)
		if err != nil {
			return nil, err
		}
		a, err := asset.New(sym, name)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
