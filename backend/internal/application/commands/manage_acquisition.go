package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/binancetracker/binancetracker/internal/application/ports"
	"github.com/binancetracker/binancetracker/internal/domain/acquisition"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/shopspring/decimal"
)

// ErrNotEditable is returned when the user tries to edit or delete an
// acquisition that did not originate from a manual entry. We refuse those to
// avoid fighting the auto-importer: the next sync would re-create the row
// anyway, leaving the user confused.
var ErrNotEditable = errors.New("only manual acquisitions can be edited or deleted")

// manualPrefix is the ID prefix every manual entry carries (see manualID in
// create_acquisition.go).
const manualPrefix = "manual-"

func isManual(id string) bool { return strings.HasPrefix(id, manualPrefix) }

// DeleteAcquisition removes a manual fund entry. Auto-imported rows are
// rejected with ErrNotEditable.
type DeleteAcquisition struct {
	repo ports.AcquisitionRepository
}

func NewDeleteAcquisition(repo ports.AcquisitionRepository) *DeleteAcquisition {
	return &DeleteAcquisition{repo: repo}
}

func (uc *DeleteAcquisition) Execute(ctx context.Context, id string) error {
	if !isManual(id) {
		return ErrNotEditable
	}
	return uc.repo.Delete(ctx, id)
}

// UpdateAcquisition replaces the asset/qty/cost/timestamp of a manual fund
// entry in place. Implementation note: there's no SQL UPDATE — we delete and
// re-insert with the same id. That keeps the repository surface tiny and
// avoids a parallel mutation path.
type UpdateAcquisition struct {
	repo    ports.AcquisitionRepository
	history ports.HistoricalPriceFeed
}

func NewUpdateAcquisition(repo ports.AcquisitionRepository, history ports.HistoricalPriceFeed) *UpdateAcquisition {
	return &UpdateAcquisition{repo: repo, history: history}
}

type UpdateAcquisitionInput struct {
	ID         string
	Asset      string
	Quote      string
	Quantity   decimal.Decimal
	UnitCost   decimal.Decimal
	AcquiredAt time.Time
}

func (uc *UpdateAcquisition) Execute(ctx context.Context, in UpdateAcquisitionInput) (acquisition.Acquisition, error) {
	if !isManual(in.ID) {
		return acquisition.Acquisition{}, ErrNotEditable
	}

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
			return acquisition.Acquisition{}, fmt.Errorf("unit cost is required")
		}
		unitCost, err = uc.history.PriceAt(ctx, assetSym, quoteSym, when)
		if err != nil {
			return acquisition.Acquisition{}, fmt.Errorf("resolve historical price: %w", err)
		}
	} else {
		unitCost, err = shared.NewMoney(in.UnitCost, quoteSym)
		if err != nil {
			return acquisition.Acquisition{}, fmt.Errorf("unit cost: %w", err)
		}
	}

	a, err := acquisition.New(acquisition.Params{
		ID:         in.ID,
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
	if err := uc.repo.Delete(ctx, in.ID); err != nil {
		return acquisition.Acquisition{}, fmt.Errorf("delete old: %w", err)
	}
	if err := uc.repo.SaveBatch(ctx, []acquisition.Acquisition{a}); err != nil {
		return acquisition.Acquisition{}, fmt.Errorf("re-insert: %w", err)
	}
	return a, nil
}
