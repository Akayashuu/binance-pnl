package binance

import (
	"context"
	"time"

	"github.com/binancetracker/binancetracker/internal/domain/acquisition"
)

// FetchEarnRewardsSince is currently a no-op. The legacy lending endpoint
// (/sapi/v1/lending/union/interestHistory) was deprecated by Binance, and
// go-binance/v2 v2.6.1 doesn't expose the replacement Simple Earn endpoint
// (/sapi/v1/simple-earn/flexible/history/rewardsRecord). Re-enable this once
// the SDK ships a wrapper, or write a custom signed HTTP call.
func (c *Client) FetchEarnRewardsSince(_ context.Context, _ time.Time) ([]acquisition.Acquisition, error) {
	return nil, nil
}
