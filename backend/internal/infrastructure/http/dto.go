package http

import (
	"context"
	"time"

	"github.com/binancetracker/binancetracker/internal/application/ports"
	"github.com/binancetracker/binancetracker/internal/application/queries"
	"github.com/binancetracker/binancetracker/internal/domain/pnl"
	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/binancetracker/binancetracker/internal/domain/trade"
	"github.com/shopspring/decimal"
)

// All API DTOs live here, isolated from the domain model. The HTTP layer is
// the only place that knows about JSON shapes.

// moneyDTO carries a monetary amount in its native currency, plus an optional
// "display" representation in a secondary currency (typically EUR for
// French-speaking users while spot trades are in USDT). The frontend renders
// both side-by-side.
type moneyDTO struct {
	Amount   string         `json:"amount"`
	Currency string         `json:"currency"`
	Display  *displayMoneyDTO `json:"display,omitempty"`
}

type displayMoneyDTO struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

type pnlDTO struct {
	Asset           string   `json:"asset"`
	Quote           string   `json:"quote"`
	HeldQuantity    string   `json:"held_quantity"`
	AverageCost     moneyDTO `json:"average_cost"`
	CurrentPrice    moneyDTO `json:"current_price"`
	MarketValue     moneyDTO `json:"market_value"`
	CostBasis       moneyDTO `json:"cost_basis"`
	UnrealizedPnL   moneyDTO `json:"unrealized_pnl"`
	RealizedPnL     moneyDTO `json:"realized_pnl"`
	TotalPnL        moneyDTO `json:"total_pnl"`
	UnrealizedPctBP int64    `json:"unrealized_pct_bp"`
}

type portfolioDTO struct {
	Quote         string    `json:"quote"`
	GeneratedAt   time.Time `json:"generated_at"`
	TotalInvested moneyDTO  `json:"total_invested"`
	TotalValue    moneyDTO  `json:"total_value"`
	UnrealizedPnL moneyDTO  `json:"unrealized_pnl"`
	RealizedPnL   moneyDTO  `json:"realized_pnl"`
	TotalPnL      moneyDTO  `json:"total_pnl"`
	Positions     []pnlDTO  `json:"positions"`
}

type tradeDTO struct {
	ID         string    `json:"id"`
	Asset      string    `json:"asset"`
	Quote      string    `json:"quote"`
	Side       string    `json:"side"`
	Source     string    `json:"source"`
	Quantity   string    `json:"quantity"`
	Price      moneyDTO  `json:"price"`
	Fee        moneyDTO  `json:"fee"`
	ExecutedAt time.Time `json:"executed_at"`
	GrossValue moneyDTO  `json:"gross_value"`

	// Per-trade P&L (only meaningful on BUY rows where trade.quote matches
	// the configured app quote).
	DeltaTotal   moneyDTO `json:"delta_total"`
	DeltaPct     string   `json:"delta_pct"`
	RemainingQty string   `json:"remaining_qty"`
	RemainingPnL moneyDTO `json:"remaining_pnl"`
}

type lotDTO struct {
	AcquisitionID     string   `json:"acquisition_id"`
	Source            string   `json:"source"`
	AcquiredAt        string   `json:"acquired_at"`
	OriginalQuantity  string   `json:"original_quantity"`
	RemainingQuantity string   `json:"remaining_quantity"`
	UnitCost          moneyDTO `json:"unit_cost"`
	CostBasis         moneyDTO `json:"cost_basis"`
	CurrentPrice      moneyDTO `json:"current_price"`
	CurrentValue      moneyDTO `json:"current_value"`
	UnrealizedPnL     moneyDTO `json:"unrealized_pnl"`
	UnrealizedPnLPct  string   `json:"unrealized_pnl_pct"`
}

type assetDetailDTO struct {
	PnL    pnlDTO     `json:"pnl"`
	Trades []tradeDTO `json:"trades"`
	Lots   []lotDTO   `json:"lots"`
}

type settingsDTO struct {
	BinanceAPIKeySet    bool   `json:"binance_api_key_set"`
	BinanceAPISecretSet bool   `json:"binance_api_secret_set"`
	QuoteCurrency       string `json:"quote_currency"`
	DisplayCurrency     string `json:"display_currency"`
}

type updateSettingsDTO struct {
	BinanceAPIKey    string `json:"binance_api_key"`
	BinanceAPISecret string `json:"binance_api_secret"`
	QuoteCurrency    string `json:"quote_currency"`
}

type syncResultDTO struct {
	Imported       int            `json:"imported"`
	BySource       map[string]int `json:"by_source"`
	PartialFailure bool           `json:"partial_failure"`
	Errors         []string       `json:"errors,omitempty"`
}

type createAcquisitionDTO struct {
	Asset      string `json:"asset"`
	Quote      string `json:"quote"`
	Quantity   string `json:"quantity"`
	UnitCost   string `json:"unit_cost"`
	AcquiredAt string `json:"acquired_at"`
}

// --- mapper -----------------------------------------------------------------

// dtoMapper carries the dependencies needed to enrich every monetary value in
// outgoing DTOs with a "display" representation in the user's secondary
// currency (e.g. EUR alongside USDT). The mapper is built per-request because
// it caches FX rates lazily during the response build — those caches must
// not leak across requests.
type dtoMapper struct {
	ctx       context.Context
	fx        ports.FxRateProvider
	displayCC shared.Symbol
}

func newDtoMapper(ctx context.Context, fx ports.FxRateProvider, displayCC shared.Symbol) *dtoMapper {
	return &dtoMapper{ctx: ctx, fx: fx, displayCC: displayCC}
}

// money turns a domain Money into the wire DTO and best-effort fills the
// secondary display amount. If FX conversion fails (e.g. an obscure altcoin
// has no Binance pair), Display is left nil and the frontend just shows the
// native value.
func (m *dtoMapper) money(in shared.Money) moneyDTO {
	out := moneyDTO{
		Amount:   in.Amount().String(),
		Currency: in.Currency().String(),
	}
	if m == nil || m.fx == nil || m.displayCC.IsZero() || in.Currency().IsZero() {
		return out
	}
	if in.Currency().Equals(m.displayCC) {
		// Already in display currency — surface it identically so the
		// frontend can rely on Display always being present when conversion
		// is possible.
		out.Display = &displayMoneyDTO{
			Amount:   in.Amount().String(),
			Currency: m.displayCC.String(),
		}
		return out
	}
	rate, err := m.fx.Rate(m.ctx, in.Currency(), m.displayCC)
	if err != nil {
		return out
	}
	converted := in.Amount().Mul(rate)
	out.Display = &displayMoneyDTO{
		Amount:   converted.String(),
		Currency: m.displayCC.String(),
	}
	return out
}

// moneyFromDecimal builds a moneyDTO from a raw decimal + currency. Used by
// per-lot and per-trade fields where the underlying value is a decimal, not
// a domain Money.
func (m *dtoMapper) moneyFromDecimal(amount decimal.Decimal, currency shared.Symbol) moneyDTO {
	mon, err := shared.NewMoney(amount, currency)
	if err != nil {
		return moneyDTO{Amount: amount.String(), Currency: currency.String()}
	}
	return m.money(mon)
}

func (m *dtoMapper) pnl(r pnl.Result) pnlDTO {
	return pnlDTO{
		Asset:           r.Asset.String(),
		Quote:           r.Quote.String(),
		HeldQuantity:    r.HeldQuantity.Decimal().String(),
		AverageCost:     m.money(r.AverageCost),
		CurrentPrice:    m.money(r.CurrentPrice),
		MarketValue:     m.money(r.MarketValue),
		CostBasis:       m.money(r.CostBasis),
		UnrealizedPnL:   m.money(r.UnrealizedPnL),
		RealizedPnL:     m.money(r.RealizedPnL),
		TotalPnL:        m.money(r.TotalPnL),
		UnrealizedPctBP: r.UnrealizedPctBP,
	}
}

func (m *dtoMapper) portfolio(o queries.PortfolioOverview) portfolioDTO {
	out := portfolioDTO{
		Quote:         o.Quote.String(),
		GeneratedAt:   o.GeneratedAt,
		TotalInvested: m.money(o.TotalInvested),
		TotalValue:    m.money(o.TotalValue),
		UnrealizedPnL: m.money(o.UnrealizedPnL),
		RealizedPnL:   m.money(o.RealizedPnL),
		TotalPnL:      m.money(o.TotalPnL),
		Positions:     make([]pnlDTO, 0, len(o.Positions)),
	}
	for _, p := range o.Positions {
		out.Positions = append(out.Positions, m.pnl(p))
	}
	return out
}

func (m *dtoMapper) tradeView(v queries.TradeView) tradeDTO {
	t := v.Trade
	return tradeDTO{
		ID:           t.ID(),
		Asset:        t.Asset().String(),
		Quote:        t.Quote().String(),
		Side:         string(t.Side()),
		Source:       string(t.Source()),
		Quantity:     t.Quantity().Decimal().String(),
		Price:        m.money(t.Price()),
		Fee:          m.money(t.Fee()),
		ExecutedAt:   t.ExecutedAt(),
		GrossValue:   m.money(t.GrossValue()),
		DeltaTotal:   m.moneyFromDecimal(v.DeltaTotal, t.Quote()),
		DeltaPct:     v.DeltaPct.StringFixed(2),
		RemainingQty: v.RemainingQty.String(),
		RemainingPnL: m.moneyFromDecimal(v.RemainingPnL, t.Quote()),
	}
}

func (m *dtoMapper) tradeViews(in []queries.TradeView) []tradeDTO {
	out := make([]tradeDTO, 0, len(in))
	for _, v := range in {
		out = append(out, m.tradeView(v))
	}
	return out
}

// trade is a fallback used by /api/v1/trades which doesn't have per-trade
// P&L computed (no asset context).
func (m *dtoMapper) trade(t trade.Trade) tradeDTO {
	return tradeDTO{
		ID:         t.ID(),
		Asset:      t.Asset().String(),
		Quote:      t.Quote().String(),
		Side:       string(t.Side()),
		Source:     string(t.Source()),
		Quantity:   t.Quantity().Decimal().String(),
		Price:      m.money(t.Price()),
		Fee:        m.money(t.Fee()),
		ExecutedAt: t.ExecutedAt(),
		GrossValue: m.money(t.GrossValue()),
	}
}

func (m *dtoMapper) trades(in []trade.Trade) []tradeDTO {
	out := make([]tradeDTO, 0, len(in))
	for _, t := range in {
		out = append(out, m.trade(t))
	}
	return out
}

func (m *dtoMapper) lot(l queries.LotView, quote shared.Symbol) lotDTO {
	return lotDTO{
		AcquisitionID:     l.AcquisitionID,
		Source:            string(l.Source),
		AcquiredAt:        l.AcquiredAt,
		OriginalQuantity:  l.OriginalQuantity.String(),
		RemainingQuantity: l.RemainingQuantity.String(),
		UnitCost:          m.moneyFromDecimal(l.UnitCost, quote),
		CostBasis:         m.moneyFromDecimal(l.CostBasis, quote),
		CurrentPrice:      m.moneyFromDecimal(l.CurrentPrice, quote),
		CurrentValue:      m.moneyFromDecimal(l.CurrentValue, quote),
		UnrealizedPnL:     m.moneyFromDecimal(l.UnrealizedPnL, quote),
		UnrealizedPnLPct:  l.UnrealizedPnLPct.StringFixed(2),
	}
}

func (m *dtoMapper) lots(in []queries.LotView, quote shared.Symbol) []lotDTO {
	out := make([]lotDTO, 0, len(in))
	for _, l := range in {
		out = append(out, m.lot(l, quote))
	}
	return out
}
