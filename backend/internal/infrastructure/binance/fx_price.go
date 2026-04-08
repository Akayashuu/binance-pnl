package binance

import (
	"context"
	"fmt"

	bn "github.com/adshao/go-binance/v2"
	"github.com/shopspring/decimal"
)

// binancePriceClient is the production implementation of bnPriceClient. It
// uses an unauthenticated SDK Client because /api/v3/ticker/price is public.
type binancePriceClient struct {
	api *bn.Client
}

func newBinancePriceClient() *binancePriceClient {
	// Empty credentials are fine: the ticker endpoint does not require auth.
	return &binancePriceClient{api: bn.NewClient("", "")}
}

// priceFor returns the latest spot ticker for `symbol` (e.g. BTCUSDT) as a
// decimal. Returns an error if Binance does not list the pair.
func (b *binancePriceClient) priceFor(ctx context.Context, symbol string) (decimal.Decimal, error) {
	res, err := b.api.NewListPricesService().Symbol(symbol).Do(ctx)
	if err != nil {
		return decimal.Zero, err
	}
	if len(res) == 0 {
		return decimal.Zero, fmt.Errorf("no ticker for %s", symbol)
	}
	return decimal.NewFromString(res[0].Price)
}
