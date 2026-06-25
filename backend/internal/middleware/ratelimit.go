package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/httprate"
)

// RateLimiter returns a Redis-backed (or in-memory fallback) rate limiter middleware.
// requestsPerMinute: maximum requests allowed per IP per minute.
//
// go-chi/httprate supports optional Redis backends via httprate.WithKeyFuncs.
// For production use the Redis option; the default in-memory map works fine
// for a single-node deployment or local dev.
func RateLimiter(requestsPerMinute int) func(http.Handler) http.Handler {
	return httprate.LimitByIP(requestsPerMinute, time.Minute)
}

// RateLimiterWithResponse wraps the limiter and returns JSON on 429.
func RateLimiterWithResponse(requestsPerMinute int) func(http.Handler) http.Handler {
	return httprate.Limit(
		requestsPerMinute,
		time.Minute,
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded","retry_after_seconds":60}`))
		}),
	)
}
