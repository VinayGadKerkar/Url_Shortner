package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"urlshortener/internal/middleware"

	"go.uber.org/zap"
)

// RouterDeps groups all handler dependencies needed to build the router.
type RouterDeps struct {
	URLHandler    *URLHandler
	HealthHandler *HealthHandler
	Logger        *zap.Logger
	// RequestsPerMinute configures the IP-based rate limiter.
	RequestsPerMinute int
}

// NewRouter wires up all routes and middleware and returns the root http.Handler.
//
// Route layout:
//
//	GET|HEAD /health                    → liveness/readiness check
//	GET      /{shortCode}               → redirect to long URL
//	POST     /api/v1/shorten            → create short URL
//	GET      /api/v1/analytics/{shortCode}  → analytics for a short code
func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	// --- Global middleware (applied to every route) ---
	r.Use(chimiddleware.RequestID)    // injects X-Request-Id header
	r.Use(chimiddleware.RealIP)       // trusts X-Forwarded-For / X-Real-IP
	r.Use(middleware.Recovery(deps.Logger))
	r.Use(middleware.RequestLogger(deps.Logger))
	r.Use(middleware.RateLimiterWithResponse(deps.RequestsPerMinute))

	// --- Health ---
	// Register both GET and HEAD so Docker/wget healthchecks work correctly.
	r.Get("/health", deps.HealthHandler.Health)
	r.Head("/health", deps.HealthHandler.Health)

	// --- Redirect (root-level, short and clean) ---
	r.Get("/{shortCode}", deps.URLHandler.Redirect)

	// --- API v1 ---
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/shorten", deps.URLHandler.CreateShortURL)
		r.Get("/analytics/{shortCode}", deps.URLHandler.GetAnalytics)
	})

	return r
}
