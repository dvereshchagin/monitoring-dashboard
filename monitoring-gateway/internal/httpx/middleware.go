package httpx

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const requestIDHeader = "X-Request-Id"

func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(requestIDHeader)
		if requestID == "" {
			requestID = uuid.NewString()
		}
		w.Header().Set(requestIDHeader, requestID)
		r.Header.Set(requestIDHeader, requestID)
		next.ServeHTTP(w, r)
	})
}

func WithLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("gateway request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
			"duration_ms", time.Since(started).Milliseconds(),
			"request_id", r.Header.Get(requestIDHeader),
		)
	})
}
