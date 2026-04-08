package commands

import (
	"context"
	"fmt"

	"github.com/binancetracker/binancetracker/internal/application/ports"
)

// Setting keys recognised by the application. They are exported so adapters
// (HTTP handlers, tests) can reference them by name. They are storage keys,
// not credentials, hence the gosec suppression.
const (
	SettingBinanceAPIKey    = "binance_api_key"    //nolint:gosec // storage key, not a secret
	SettingBinanceAPISecret = "binance_api_secret" //nolint:gosec // storage key, not a secret
	SettingQuoteCurrency    = "quote_currency"
)

// Encryptor protects sensitive setting values at rest. The application layer
// uses an interface so the concrete crypto implementation lives in
// `internal/crypto`.
type Encryptor interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

// SaveSettings persists user-supplied settings, encrypting sensitive values.
type SaveSettings struct {
	repo      ports.SettingsRepository
	encryptor Encryptor
}

// NewSaveSettings wires the use case.
func NewSaveSettings(repo ports.SettingsRepository, encryptor Encryptor) *SaveSettings {
	return &SaveSettings{repo: repo, encryptor: encryptor}
}

// Input is the structured payload for the use case.
type Input struct {
	BinanceAPIKey    string
	BinanceAPISecret string
	QuoteCurrency    string
}

// Execute saves all non-empty fields. Empty fields are left untouched.
func (uc *SaveSettings) Execute(ctx context.Context, in Input) error {
	if in.BinanceAPIKey != "" {
		enc, err := uc.encryptor.Encrypt(in.BinanceAPIKey)
		if err != nil {
			return fmt.Errorf("encrypt api key: %w", err)
		}
		if err := uc.repo.Set(ctx, SettingBinanceAPIKey, enc); err != nil {
			return err
		}
	}
	if in.BinanceAPISecret != "" {
		enc, err := uc.encryptor.Encrypt(in.BinanceAPISecret)
		if err != nil {
			return fmt.Errorf("encrypt api secret: %w", err)
		}
		if err := uc.repo.Set(ctx, SettingBinanceAPISecret, enc); err != nil {
			return err
		}
	}
	if in.QuoteCurrency != "" {
		if err := uc.repo.Set(ctx, SettingQuoteCurrency, in.QuoteCurrency); err != nil {
			return err
		}
	}
	return nil
}
