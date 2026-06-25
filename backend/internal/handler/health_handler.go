package handler

import (
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"

	"urlshortener/internal/repository"
)

// HealthHandler exposes a /health endpoint that checks all downstream dependencies.
type HealthHandler struct {
	cache  repository.CacheRepository
	logger *zap.Logger
	// dbPing is a lightweight function that pings the DB pool.
	dbPing func(ctx context.Context) error
}

// NewHealthHandler constructs a HealthHandler.
func NewHealthHandler(cache repository.CacheRepository, dbPing func(ctx context.Context) error, logger *zap.Logger) *HealthHandler {
	return &HealthHandler{cache: cache, dbPing: dbPing, logger: logger}
}

type healthStatus struct {
	Status     string            `json:"status"`
	Components map[string]string `json:"components"`
}

// Health handles GET /health
// Returns 200 if all dependencies are reachable, 503 otherwise.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	components := make(map[string]string)
	healthy := true

	// Check PostgreSQL.
	if err := h.dbPing(ctx); err != nil {
		h.logger.Warn("health check: postgres down", zap.Error(err))
		components["postgres"] = "unhealthy"
		healthy = false
	} else {
		components["postgres"] = "healthy"
	}

	// Check Redis.
	if err := h.cache.Ping(ctx); err != nil {
		h.logger.Warn("health check: redis down", zap.Error(err))
		components["redis"] = "unhealthy"
		healthy = false
	} else {
		components["redis"] = "healthy"
	}

	status := healthStatus{Components: components}
	if healthy {
		status.Status = "ok"
		writeJSON(w, http.StatusOK, status)
	} else {
		status.Status = "degraded"
		writeJSON(w, http.StatusServiceUnavailable, status)
	}
}
