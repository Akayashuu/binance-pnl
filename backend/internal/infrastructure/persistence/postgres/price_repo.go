package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/binancetracker/binancetracker/internal/application/ports"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

var _ ports.PriceRepository = (*PriceRepo)(nil)

// PriceRepo caches latest market prices per asset.
type PriceRepo struct {
	pool  *pgxpool.Pool
	quote shared.Symbol
}

// NewPriceRepo wires the repository. The quote currency is fixed at startup
// (e.g. USDT) — switching it requires re-syncing prices.
func NewPriceRepo(pool *pgxpool.Pool, quote shared.Symbol) *PriceRepo {
	return &PriceRepo{pool: pool, quote: quote}
}

// Upsert stores the latest price for an asset.
func (r *PriceRepo) Upsert(ctx context.Context, symbol shared.Symbol, price shared.Money, fetchedAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO price_quotes (asset, price, quote, fetched_at)
		 VALUES ($1,$2,$3,$4)
		 ON CONFLICT (asset) DO UPDATE
		   SET price = EXCLUDED.price,
		       quote = EXCLUDED.quote,
		       fetched_at = EXCLUDED.fetched_at`,
		symbol.String(), price.Amount().String(), price.Currency().String(), fetchedAt.UTC())
	return err
}

// Latest returns the cached price for an asset.
func (r *PriceRepo) Latest(ctx context.Context, symbol shared.Symbol) (shared.Money, time.Time, error) {
	var (
		amount    decimal.Decimal
		quote     string
		fetchedAt time.Time
	)
	err := r.pool.QueryRow(ctx,
		`SELECT price, quote, fetched_at FROM price_quotes WHERE asset = $1`,
		symbol.String()).Scan(&amount, &quote, &fetchedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return shared.Money{}, time.Time{}, err
		}
		return shared.Money{}, time.Time{}, err
	}
	quoteSym, err := shared.NewSymbol(quote)
	if err != nil {
		return shared.Money{}, time.Time{}, err
	}
	m, err := shared.NewMoney(amount, quoteSym)
	if err != nil {
		return shared.Money{}, time.Time{}, err
	}
	return m, fetchedAt, nil
}

// LatestMany returns cached prices for many assets.
func (r *PriceRepo) LatestMany(ctx context.Context, symbols []shared.Symbol) (map[shared.Symbol]shared.Money, error) {
	if len(symbols) == 0 {
		return map[shared.Symbol]shared.Money{}, nil
	}
	args := make([]any, len(symbols))
	for i, s := range symbols {
		args[i] = s.String()
	}

	rows, err := r.pool.Query(ctx,
		`SELECT asset, price, quote FROM price_quotes WHERE asset = ANY($1)`,
		args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[shared.Symbol]shared.Money, len(symbols))
	for rows.Next() {
		var assetStr, quoteStr string
		var price decimal.Decimal
		if err := rows.Scan(&assetStr, &price, &quoteStr); err != nil {
			return nil, err
		}
		assetSym, err := shared.NewSymbol(assetStr)
		if err != nil {
			return nil, err
		}
		quoteSym, err := shared.NewSymbol(quoteStr)
		if err != nil {
			return nil, err
		}
		m, err := shared.NewMoney(price, quoteSym)
		if err != nil {
			return nil, err
		}
		out[assetSym] = m
	}
	return out, rows.Err()
}
