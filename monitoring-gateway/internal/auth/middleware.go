package auth

import (
	"net/http"
	"strings"

	gatewaymetrics "github.com/dreschagin/monitoring-dashboard/monitoring-gateway/internal/metrics"
)

var unauthenticatedPaths = map[string]struct{}{
	"/healthz": {},
	"/readyz":  {},
	"/metrics": {},
}

// Middleware validates bearer token for protected routes.
func Middleware(enabled bool, bearerToken string, metrics *gatewaymetrics.Metrics, next http.Handler) http.Handler {
	if !enabled {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := unauthenticatedPaths[r.URL.Path]; ok {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			metrics.AuthFailures.Inc()
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if token == "" || token != bearerToken {
			metrics.AuthFailures.Inc()
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		r.Header.Set("X-Auth-Subject", "gateway-shared-token")
		next.ServeHTTP(w, r)
	})
}
