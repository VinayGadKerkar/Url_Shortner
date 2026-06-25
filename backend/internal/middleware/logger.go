package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// RequestLogger returns middleware that logs every request using structured zap logging.
// Logs: method, path, status, latency, remote address.
// Health check probes from internal sources (Wget/wget) are suppressed to reduce noise.
func RequestLogger(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			// Skip logging internal Docker health check probes — they fire every
			// 15 seconds and add nothing useful to the log stream.
			isHealthProbe := r.URL.Path == "/health" &&
				(r.Method == http.MethodHead || r.Method == http.MethodGet) &&
				(r.UserAgent() == "" || len(r.UserAgent()) >= 4 && r.UserAgent()[:4] == "Wget")
			if isHealthProbe {
				return
			}

			logger.Info("request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", wrapped.status),
				zap.Duration("latency", time.Since(start)),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()),
			)
		})
	}
}
