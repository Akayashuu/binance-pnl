package commands

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/binancetracker/binancetracker/internal/application/ports"
	"github.com/binancetracker/binancetracker/internal/domain/acquisition"
	"github.com/binancetracker/binancetracker/internal/domain/asset"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/shopspring/decimal"
)

// CreateAcquisition lets the user record a "fund" by hand: an asset hitting
// the account from a source the auto-importer cannot see (an external
// transfer, an OTC buy, a hardware wallet top-up). It is persisted as a
// regular Acquisition with source=deposit so it flows through the same lot
// machinery as automatic imports.
type CreateAcquisition struct {
	repo    ports.AcquisitionRepository
	assets  ports.AssetRepository
	history ports.HistoricalPriceFeed
	logger  ports.Logger
}

func NewCreateAcquisition(
	repo ports.AcquisitionRepository,
	assets ports.AssetRepository,
	history ports.HistoricalPriceFeed,
	logger ports.Logger,
) *CreateAcquisition {
	return &CreateAcquisition{repo: repo, assets: assets, history: history, logger: logger}
}

type CreateAcquisitionInput struct {
	Asset      string
	Quote      string
	Quantity   decimal.Decimal
	// UnitCost is optional. When zero (or nil), the use case asks the
	// HistoricalPriceFeed for the spot price at AcquiredAt and uses that as
	// the cost basis. This is the "I know when I got the coins but not the
	// price" workflow.
	UnitCost   decimal.Decimal
	AcquiredAt time.Time
}

func (uc *CreateAcquisition) Execute(ctx context.Context, in CreateAcquisitionInput) (acquisition.Acquisition, error) {
	assetSym, err := shared.NewSymbol(in.Asset)
	if err != nil {
		return acquisition.Acquisition{}, fmt.Errorf("asset: %w", err)
	}
	quoteSym, err := shared.NewSymbol(in.Quote)
	if err != nil {
		return acquisition.Acquisition{}, fmt.Errorf("quote: %w", err)
	}
	qty, err := shared.NewQuantity(in.Quantity)
	if err != nil {
		return acquisition.Acquisition{}, fmt.Errorf("quantity: %w", err)
	}
	when := in.AcquiredAt
	if when.IsZero() {
		when = time.Now()
	}

	var unitCost shared.Money
	if in.UnitCost.IsZero() {
		if uc.history == nil {
			return acquisition.Acquisition{}, fmt.Errorf("unit cost is required (no historical price feed configured)")
		}
		unitCost, err = uc.history.PriceAt(ctx, assetSym, quoteSym, when)
		if err != nil {
			return acquisition.Acquisition{}, fmt.Errorf("resolve historical price for %s@%s: %w",
				assetSym, when.Format(time.RFC3339), err)
		}
	} else {
		unitCost, err = shared.NewMoney(in.UnitCost, quoteSym)
		if err != nil {
			return acquisition.Acquisition{}, fmt.Errorf("unit cost: %w", err)
		}
	}

	id, err := manualID()
	if err != nil {
		return acquisition.Acquisition{}, err
	}
	a, err := acquisition.New(acquisition.Params{
		ID:         id,
		Asset:      assetSym,
		Quote:      quoteSym,
		Source:     shared.SourceDeposit,
		Quantity:   qty,
		UnitCost:   unitCost,
		AcquiredAt: when,
	})
	if err != nil {
		return acquisition.Acquisition{}, err
	}

	if err := uc.repo.SaveBatch(ctx, []acquisition.Acquisition{a}); err != nil {
		return acquisition.Acquisition{}, fmt.Errorf("persist: %w", err)
	}

	if assetEntity, err := asset.New(assetSym, assetSym.String()); err == nil {
		if err := uc.assets.Upsert(ctx, assetEntity); err != nil {
			uc.logger.Warn("upsert asset failed", "symbol", assetSym.String(), "err", err)
		}
	}
	return a, nil
}

// manualID generates a deterministic-looking but unique ID for a manual
// entry. The "manual-" prefix lets us identify hand-entered rows in raw SQL
// and won't ever collide with auto-imported deposit txids.
func manualID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "manual-" + hex.EncodeToString(b[:]), nil
}
