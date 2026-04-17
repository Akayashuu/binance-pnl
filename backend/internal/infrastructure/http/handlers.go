// Package http exposes the application use cases over a REST API. The
// handlers depend on the application use case structs (themselves wired
// against ports), never on infrastructure types.
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/binancetracker/binancetracker/internal/application/commands"
	"github.com/binancetracker/binancetracker/internal/application/ports"
	"github.com/binancetracker/binancetracker/internal/application/queries"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/binancetracker/binancetracker/internal/infrastructure/binance"
	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"
)

// Handlers groups the HTTP endpoints. Use case fields are application
// structs — concrete but injected, so we can build the handler with mocks in
// integration tests.
type Handlers struct {
	Sync              *commands.SyncBinanceTrades
	GetPortfolio      *queries.GetPortfolio
	ListTrades        *queries.ListTrades
	GetAssetDetail    *queries.GetAssetDetail
	SaveSettings      *commands.SaveSettings
	CreateAcquisition *commands.CreateAcquisition
	UpdateAcquisition *commands.UpdateAcquisition
	DeleteAcquisition *commands.DeleteAcquisition
	SettingsRepo      ports.SettingsRepository
	Binance           *binance.Client
	Fx                ports.FxRateProvider
	DisplayCurrency   shared.Symbol
	QuoteCurrency     shared.Symbol
}

// mapper builds a per-request DTO mapper. The display currency comes from the
// `display` query param when present (so the user can switch EUR/USD/GBP/…
// from the UI without a server restart) and falls back to the env-configured
// default otherwise.
func (h *Handlers) mapper(r *http.Request) *dtoMapper {
	display := h.DisplayCurrency
	if raw := r.URL.Query().Get("display"); raw != "" {
		if sym, err := shared.NewSymbol(raw); err == nil {
			display = sym
		}
	}
	return newDtoMapper(r.Context(), h.Fx, display)
}

// healthz returns 200 OK for liveness probes.
func healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handlers) sync(w http.ResponseWriter, r *http.Request) {
	full := r.URL.Query().Get("full") == "true"
	res, err := h.Sync.Execute(r.Context(), full)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, syncResultDTO{
		Imported:       res.Imported,
		BySource:       res.BySource,
		PartialFailure: res.PartialFailure,
		Errors:         res.Errors,
	})
}

func (h *Handlers) getPortfolio(w http.ResponseWriter, r *http.Request) {
	o, err := h.GetPortfolio.Execute(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, h.mapper(r).portfolio(o))
}

func (h *Handlers) listTrades(w http.ResponseWriter, r *http.Request) {
	filter := r.URL.Query().Get("asset")
	trades, err := h.ListTrades.Execute(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, h.mapper(r).trades(trades))
}

func (h *Handlers) getAssetDetail(w http.ResponseWriter, r *http.Request) {
	sym := chi.URLParam(r, "symbol")
	d, err := h.GetAssetDetail.Execute(r.Context(), sym)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	m := h.mapper(r)
	writeJSON(w, http.StatusOK, assetDetailDTO{
		PnL:    m.pnl(d.PnL),
		Trades: m.tradeViews(d.TradeViews),
		Lots:   m.lots(d.Lots, h.QuoteCurrency),
	})
}

func (h *Handlers) getSettings(w http.ResponseWriter, r *http.Request) {
	apiKey, _ := h.SettingsRepo.Get(r.Context(), commands.SettingBinanceAPIKey)
	apiSecret, _ := h.SettingsRepo.Get(r.Context(), commands.SettingBinanceAPISecret)
	quote, _ := h.SettingsRepo.Get(r.Context(), commands.SettingQuoteCurrency)

	writeJSON(w, http.StatusOK, settingsDTO{
		BinanceAPIKeySet:    apiKey != "",
		BinanceAPISecretSet: apiSecret != "",
		QuoteCurrency:       quote,
		DisplayCurrency:     h.DisplayCurrency.String(),
	})
}

func (h *Handlers) createAcquisition(w http.ResponseWriter, r *http.Request) {
	var in createAcquisitionDTO
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	qty, err := decimal.NewFromString(in.Quantity)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	// Empty unit_cost means "look it up at the given timestamp" — leave the
	// decimal at its zero value and let the use case resolve it.
	var unitCost decimal.Decimal
	if in.UnitCost != "" {
		unitCost, err = decimal.NewFromString(in.UnitCost)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	when := time.Time{}
	if in.AcquiredAt != "" {
		when, err = time.Parse(time.RFC3339, in.AcquiredAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	quote := in.Quote
	if quote == "" {
		quote = h.QuoteCurrency.String()
	}

	a, err := h.CreateAcquisition.Execute(r.Context(), commands.CreateAcquisitionInput{
		Asset:      in.Asset,
		Quote:      quote,
		Quantity:   qty,
		UnitCost:   unitCost,
		AcquiredAt: when,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": a.ID()})
}

func (h *Handlers) updateAcquisition(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var in createAcquisitionDTO
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	qty, err := decimal.NewFromString(in.Quantity)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var unitCost decimal.Decimal
	if in.UnitCost != "" {
		unitCost, err = decimal.NewFromString(in.UnitCost)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	when := time.Time{}
	if in.AcquiredAt != "" {
		when, err = time.Parse(time.RFC3339, in.AcquiredAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	quote := in.Quote
	if quote == "" {
		quote = h.QuoteCurrency.String()
	}

	a, err := h.UpdateAcquisition.Execute(r.Context(), commands.UpdateAcquisitionInput{
		ID:         id,
		Asset:      in.Asset,
		Quote:      quote,
		Quantity:   qty,
		UnitCost:   unitCost,
		AcquiredAt: when,
	})
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, commands.ErrNotEditable) {
			status = http.StatusForbidden
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": a.ID()})
}

func (h *Handlers) deleteAcquisition(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.DeleteAcquisition.Execute(r.Context(), id); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, commands.ErrNotEditable) {
			status = http.StatusForbidden
		}
		writeError(w, status, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) updateSettings(w http.ResponseWriter, r *http.Request) {
	var in updateSettingsDTO
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.SaveSettings.Execute(r.Context(), commands.Input{
		BinanceAPIKey:    in.BinanceAPIKey,
		BinanceAPISecret: in.BinanceAPISecret,
		QuoteCurrency:    in.QuoteCurrency,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) getKlines(w http.ResponseWriter, r *http.Request) {
	sym := chi.URLParam(r, "symbol")
	if _, err := shared.NewSymbol(sym); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "1d"
	}
	limit := 90
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	asset, _ := shared.NewSymbol(sym)
	klines, err := h.Binance.FetchKlines(r.Context(), asset, interval, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	type klineDTO struct {
		Time   int64  `json:"time"`
		Open   string `json:"open"`
		High   string `json:"high"`
		Low    string `json:"low"`
		Close  string `json:"close"`
		Volume string `json:"volume"`
	}
	out := make([]klineDTO, 0, len(klines))
	for _, k := range klines {
		out = append(out, klineDTO{
			Time:   k.OpenTime,
			Open:   k.Open.String(),
			High:   k.High.String(),
			Low:    k.Low.String(),
			Close:  k.Close.String(),
			Volume: k.Volume.String(),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// --- helpers ---------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
