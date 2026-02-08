package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	gatewaymetrics "github.com/dreschagin/monitoring-dashboard/monitoring-gateway/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

func TestMiddleware(t *testing.T) {
	metrics := gatewaymetrics.New(prometheus.NewRegistry())
	handler := Middleware(true, "secret-token", metrics, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		token      string
		path       string
		wantStatus int
	}{
		{name: "valid token", token: "Bearer secret-token", path: "/api/v1/metrics/history", wantStatus: http.StatusOK},
		{name: "missing token", token: "", path: "/api/v1/metrics/history", wantStatus: http.StatusUnauthorized},
		{name: "invalid token", token: "Bearer wrong", path: "/api/v1/metrics/history", wantStatus: http.StatusUnauthorized},
		{name: "health without token", token: "", path: "/healthz", wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.token != "" {
				req.Header.Set("Authorization", tt.token)
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}
