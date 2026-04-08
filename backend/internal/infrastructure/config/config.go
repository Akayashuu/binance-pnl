// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/binancetracker/binancetracker/internal/domain/shared"
)

// Config is the runtime configuration of the API.
type Config struct {
	HTTPPort             string
	DatabaseURL          string
	LogLevel             string
	EncryptionKey        string
	BinanceAPIKey        string
	BinanceAPISecret     string
	PriceRefreshInterval time.Duration
	QuoteCurrency        shared.Symbol
	// DisplayCurrency is the secondary currency every monetary value is also
	// rendered in (alongside its native currency). Used for the dual-money UX.
	// Default: EUR.
	DisplayCurrency shared.Symbol
	TrackedAssets   []shared.Symbol
}

// Load reads configuration from the environment and validates required
// fields. It is called once at startup from main.
func Load() (Config, error) {
	cfg := Config{
		HTTPPort:         envOr("HTTP_PORT", "8080"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		LogLevel:         envOr("LOG_LEVEL", "info"),
		EncryptionKey:    os.Getenv("ENCRYPTION_KEY"),
		BinanceAPIKey:    os.Getenv("BINANCE_API_KEY"),
		BinanceAPISecret: os.Getenv("BINANCE_API_SECRET"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.EncryptionKey == "" {
		return Config{}, errors.New("ENCRYPTION_KEY is required (base64 32 bytes)")
	}

	rawInterval := envOr("PRICE_REFRESH_INTERVAL", "60s")
	d, err := time.ParseDuration(rawInterval)
	if err != nil {
		return Config{}, fmt.Errorf("invalid PRICE_REFRESH_INTERVAL: %w", err)
	}
	cfg.PriceRefreshInterval = d

	quote, err := shared.NewSymbol(envOr("QUOTE_CURRENCY", "USDT"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid QUOTE_CURRENCY: %w", err)
	}
	cfg.QuoteCurrency = quote

	display, err := shared.NewSymbol(envOr("DISPLAY_CURRENCY", "EUR"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid DISPLAY_CURRENCY: %w", err)
	}
	cfg.DisplayCurrency = display

	tracked := envOr("TRACKED_ASSETS", "BTC,ETH")
	for _, raw := range strings.Split(tracked, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		s, err := shared.NewSymbol(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid TRACKED_ASSETS entry %q: %w", raw, err)
		}
		cfg.TrackedAssets = append(cfg.TrackedAssets, s)
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
