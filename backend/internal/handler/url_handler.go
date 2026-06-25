package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"urlshortener/internal/models"
	"urlshortener/internal/repository"
	"urlshortener/internal/service"
	kafkapkg "urlshortener/kafka"
)

// URLHandler holds dependencies for URL-related HTTP handlers.
// Handlers are thin — they decode requests, call services, encode responses.
type URLHandler struct {
	svc      *service.URLService
	producer *kafkapkg.Producer
	logger   *zap.Logger
}

// NewURLHandler constructs a URLHandler.
func NewURLHandler(svc *service.URLService, producer *kafkapkg.Producer, logger *zap.Logger) *URLHandler {
	return &URLHandler{svc: svc, producer: producer, logger: logger}
}

// CreateShortURL handles POST /api/v1/shorten
//
//	Request  body: { "long_url": "...", "custom_alias": "...", "expires_in_hours": 24 }
//	Response body: CreateURLResponse
func (h *URLHandler) CreateShortURL(w http.ResponseWriter, r *http.Request) {
	var req models.CreateURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.svc.CreateShortURL(r.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrShortCodeConflict):
			writeError(w, http.StatusConflict, "custom alias already in use")
		case isValidationError(err):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			h.logger.Error("create short url failed", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "failed to create short URL")
		}
		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

// Redirect handles GET /{shortCode}
// Implements cache-aside: service checks Redis → PostgreSQL.
// Publishes a ClickEvent to Kafka after a successful lookup.
func (h *URLHandler) Redirect(w http.ResponseWriter, r *http.Request) {
	shortCode := chi.URLParam(r, "shortCode")

	url, err := h.svc.Resolve(r.Context(), shortCode)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			writeError(w, http.StatusNotFound, "short URL not found")
		case errors.Is(err, service.ErrURLExpired):
			writeError(w, http.StatusGone, "short URL has expired")
		default:
			h.logger.Error("redirect resolve failed", zap.Error(err), zap.String("short_code", shortCode))
			writeError(w, http.StatusInternalServerError, "failed to resolve URL")
		}
		return
	}

	// Publish click event to Kafka for async analytics processing.
	// Runs in a goroutine so Kafka latency never affects the redirect.
	// Uses a detached context (not the request context) so the write isn't
	// cancelled when the HTTP handler returns.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		event := models.ClickEvent{
			ShortCode:  shortCode,
			AccessedAt: time.Now().UTC(),
			IPAddress:  r.RemoteAddr,
			UserAgent:  r.UserAgent(),
		}
		if err := h.producer.PublishClickEvent(ctx, event); err != nil {
			h.logger.Warn("click event publish failed",
				zap.Error(err),
				zap.String("short_code", shortCode),
			)
		}
	}()

	http.Redirect(w, r, url.LongURL, http.StatusFound)
}

// GetAnalytics handles GET /api/v1/analytics/{shortCode}
func (h *URLHandler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	shortCode := chi.URLParam(r, "shortCode")

	analytics, err := h.svc.GetAnalytics(r.Context(), shortCode)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "short URL not found")
			return
		}
		h.logger.Error("get analytics failed", zap.Error(err), zap.String("short_code", shortCode))
		writeError(w, http.StatusInternalServerError, "failed to retrieve analytics")
		return
	}

	writeJSON(w, http.StatusOK, analytics)
}

// isValidationError is a simple heuristic — validation errors are wrapped
// with the "validation:" prefix by the service layer.
func isValidationError(err error) bool {
	if err == nil {
		return false
	}
	// errors.As could be used with a custom type; prefix check is simpler here.
	return len(err.Error()) >= 11 && err.Error()[:11] == "validation:"
}
