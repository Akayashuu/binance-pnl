// Command api is the binancetracker HTTP server.
//
// This is the composition root of the application: the only place where
// concrete adapters from `internal/infrastructure` are instantiated and
// wired into the use cases.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/binancetracker/binancetracker/internal/application/commands"
	"github.com/binancetracker/binancetracker/internal/application/queries"
	"github.com/binancetracker/binancetracker/internal/crypto"
	"github.com/binancetracker/binancetracker/internal/infrastructure/binance"
	"github.com/binancetracker/binancetracker/internal/infrastructure/config"
	httpadapter "github.com/binancetracker/binancetracker/internal/infrastructure/http"
	"github.com/binancetracker/binancetracker/internal/infrastructure/persistence/postgres"
	"github.com/binancetracker/binancetracker/internal/infrastructure/scheduler"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := httpadapter.NewLogger(cfg.LogLevel)
	logger.Info("starting binancetracker", "port", cfg.HTTPPort)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --- migrations ---------------------------------------------------------
	if err := postgres.RunMigrations(cfg.DatabaseURL); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	// --- driven adapters ----------------------------------------------------
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	encryptor, err := crypto.NewAESGCM(cfg.EncryptionKey)
	if err != nil {
		return fmt.Errorf("init encryptor: %w", err)
	}

	tradeRepo := postgres.NewTradeRepo(pool)
	acquisitionRepo := postgres.NewAcquisitionRepo(pool)
	assetRepo := postgres.NewAssetRepo(pool)
	priceRepo := postgres.NewPriceRepo(pool, cfg.QuoteCurrency)
	settingsRepo := postgres.NewSettingsRepo(pool)

	binanceClient := binance.New(binance.Config{
		APIKey:         resolveSecret(ctx, settingsRepo, encryptor, commands.SettingBinanceAPIKey, cfg.BinanceAPIKey, logger),
		APISecret:      resolveSecret(ctx, settingsRepo, encryptor, commands.SettingBinanceAPISecret, cfg.BinanceAPISecret, logger),
		QuoteAsset:     cfg.QuoteCurrency,
		AcceptedQuotes: cfg.AcceptedQuotes,
		TrackedBases:   cfg.TrackedAssets,
	})
	fxClient := binance.NewFxClient()

	// --- use cases ----------------------------------------------------------
	syncUC := commands.NewSyncBinanceTrades(binanceClient, tradeRepo, acquisitionRepo, assetRepo, logger)
	refreshUC := commands.NewRefreshPrices(binanceClient, assetRepo, priceRepo, logger)
	saveSettingsUC := commands.NewSaveSettings(settingsRepo, encryptor)
	createAcquisitionUC := commands.NewCreateAcquisition(acquisitionRepo, assetRepo, binanceClient, logger)
	updateAcquisitionUC := commands.NewUpdateAcquisition(acquisitionRepo, binanceClient)
	deleteAcquisitionUC := commands.NewDeleteAcquisition(acquisitionRepo)
	getPortfolioUC := queries.NewGetPortfolio(tradeRepo, acquisitionRepo, priceRepo, fxClient, cfg.QuoteCurrency, cfg.AcceptedQuotes)
	listTradesUC := queries.NewListTrades(tradeRepo, acquisitionRepo, cfg.QuoteCurrency)
	getAssetDetailUC := queries.NewGetAssetDetail(tradeRepo, acquisitionRepo, priceRepo, fxClient, cfg.QuoteCurrency)

	handlers := &httpadapter.Handlers{
		Sync:              syncUC,
		GetPortfolio:      getPortfolioUC,
		ListTrades:        listTradesUC,
		GetAssetDetail:    getAssetDetailUC,
		SaveSettings:      saveSettingsUC,
		CreateAcquisition: createAcquisitionUC,
		UpdateAcquisition: updateAcquisitionUC,
		DeleteAcquisition: deleteAcquisitionUC,
		SettingsRepo:      settingsRepo,
		Binance:           binanceClient,
		Fx:                fxClient,
		DisplayCurrency:   cfg.DisplayCurrency,
		QuoteCurrency:     cfg.QuoteCurrency,
	}

	// --- background jobs ----------------------------------------------------
	refresher := scheduler.NewPriceRefresher(refreshUC, cfg.PriceRefreshInterval, logger)
	go refresher.Run(ctx)

	// --- HTTP server --------------------------------------------------------
	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           httpadapter.NewRouter(handlers),
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-serverErr:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	logger.Info("bye")
	return nil
}

// resolveSecret prefers the value stored in the settings table (encrypted),
// falling back to the env var if no DB value exists. Empty string is OK at
// startup — the user can configure credentials via the Settings UI.
func resolveSecret(
	ctx context.Context,
	repo *postgres.SettingsRepo,
	enc *crypto.AESGCM,
	key string,
	envFallback string,
	logger *httpadapter.Logger,
) string {
	stored, err := repo.Get(ctx, key)
	if err != nil {
		logger.Warn("read setting failed", "key", key, "err", err)
		return envFallback
	}
	if stored == "" {
		return envFallback
	}
	pt, err := enc.Decrypt(stored)
	if err != nil {
		logger.Warn("decrypt setting failed", "key", key, "err", err)
		return envFallback
	}
	return pt
}
