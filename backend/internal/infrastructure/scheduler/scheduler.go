// Package scheduler runs background jobs (price refresh) on a fixed cadence.
package scheduler

import (
	"context"
	"time"

	"github.com/binancetracker/binancetracker/internal/application/commands"
	"github.com/binancetracker/binancetracker/internal/application/ports"
)

// PriceRefresher invokes the RefreshPrices use case at a configured cadence.
// It runs until its context is cancelled.
type PriceRefresher struct {
	uc       *commands.RefreshPrices
	interval time.Duration
	logger   ports.Logger
}

// NewPriceRefresher wires the scheduler.
func NewPriceRefresher(uc *commands.RefreshPrices, interval time.Duration, logger ports.Logger) *PriceRefresher {
	return &PriceRefresher{uc: uc, interval: interval, logger: logger}
}

// Run blocks until the context is cancelled. It executes one initial refresh
// immediately, then ticks every `interval`.
func (s *PriceRefresher) Run(ctx context.Context) {
	if n, err := s.uc.Execute(ctx); err != nil {
		s.logger.Warn("initial price refresh failed", "err", err)
	} else {
		s.logger.Info("initial price refresh", "updated", n)
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("price refresher stopping")
			return
		case <-ticker.C:
			n, err := s.uc.Execute(ctx)
			if err != nil {
				s.logger.Warn("price refresh failed", "err", err)
				continue
			}
			s.logger.Info("price refresh", "updated", n)
		}
	}
}
