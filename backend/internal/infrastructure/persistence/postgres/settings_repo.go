package postgres

import (
	"context"
	"errors"

	"github.com/binancetracker/binancetracker/internal/application/ports"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ ports.SettingsRepository = (*SettingsRepo)(nil)

// SettingsRepo persists user-editable key/value settings.
type SettingsRepo struct {
	pool *pgxpool.Pool
}

// NewSettingsRepo wires the repository.
func NewSettingsRepo(pool *pgxpool.Pool) *SettingsRepo { return &SettingsRepo{pool: pool} }

// Get returns a setting value, or empty string with no error if absent.
func (r *SettingsRepo) Get(ctx context.Context, key string) (string, error) {
	var v string
	err := r.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = $1`, key).Scan(&v)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return v, nil
}

// Set upserts a setting.
func (r *SettingsRepo) Set(ctx context.Context, key, value string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO settings (key, value) VALUES ($1,$2)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		key, value)
	return err
}
