package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// NewRouter assembles a chi router with the standard middleware stack and
// mounts all binancetracker HTTP endpoints.
func NewRouter(h *Handlers) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(60 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/healthz", healthz)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/sync", h.sync)
		r.Get("/portfolio", h.getPortfolio)
		r.Get("/trades", h.listTrades)
		r.Get("/assets/{symbol}", h.getAssetDetail)
		r.Get("/klines/{symbol}", h.getKlines)
		r.Post("/acquisitions", h.createAcquisition)
		r.Put("/acquisitions/{id}", h.updateAcquisition)
		r.Delete("/acquisitions/{id}", h.deleteAcquisition)
		r.Get("/settings", h.getSettings)
		r.Put("/settings", h.updateSettings)
	})

	return r
}
