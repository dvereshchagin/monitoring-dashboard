package metrics

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics bundles prometheus collectors used by gateway.
type Metrics struct {
	RequestsTotal      *prometheus.CounterVec
	RequestDurationSec *prometheus.HistogramVec
	UpstreamErrors     prometheus.Counter
	AuthFailures       prometheus.Counter
	RateLimitDropped   prometheus.Counter
	DiscoveryRefreshes prometheus.Counter
	DiscoveryErrors    prometheus.Counter
}

func New(registry *prometheus.Registry) *Metrics {
	m := &Metrics{
		RequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_requests_total",
			Help: "Total number of gateway HTTP requests.",
		}, []string{"route", "method", "status"}),
		RequestDurationSec: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "gateway_request_duration_seconds",
			Help:    "Gateway request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"route", "method", "status"}),
		UpstreamErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_upstream_errors_total",
			Help: "Total number of upstream proxy errors.",
		}),
		AuthFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_auth_failures_total",
			Help: "Total number of auth failures.",
		}),
		RateLimitDropped: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_ratelimit_dropped_total",
			Help: "Total number of requests dropped by rate limiter.",
		}),
		DiscoveryRefreshes: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_discovery_refresh_total",
			Help: "Total number of discovery refresh attempts.",
		}),
		DiscoveryErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_discovery_errors_total",
			Help: "Total number of discovery refresh failures.",
		}),
	}

	registry.MustRegister(
		m.RequestsTotal,
		m.RequestDurationSec,
		m.UpstreamErrors,
		m.AuthFailures,
		m.RateLimitDropped,
		m.DiscoveryRefreshes,
		m.DiscoveryErrors,
	)

	return m
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		wrapped := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		status := strconv.Itoa(wrapped.statusCode)
		route := normalizeRoute(r.URL.Path)
		m.RequestsTotal.WithLabelValues(route, r.Method, status).Inc()
		m.RequestDurationSec.WithLabelValues(route, r.Method, status).Observe(time.Since(startedAt).Seconds())
	})
}

func normalizeRoute(path string) string {
	switch {
	case path == "/ws":
		return "/ws"
	case path == "/api/v1/release-analyzer" || hasPrefix(path, "/api/v1/release-analyzer/"):
		return "/api/v1/release-analyzer/*"
	case path == "/api/v1" || hasPrefix(path, "/api/v1/"):
		return "/api/v1/*"
	case path == "/api" || hasPrefix(path, "/api/"):
		return "/api/*"
	default:
		return "other"
	}
}

func hasPrefix(value, prefix string) bool {
	if len(value) < len(prefix) {
		return false
	}
	return value[:len(prefix)] == prefix
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rw *statusRecorder) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Hijack passes websocket upgrades through wrapped ResponseWriter.
func (rw *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

// Flush keeps streaming behavior for handlers that require it.
func (rw *statusRecorder) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Push proxies HTTP/2 server push when available.
func (rw *statusRecorder) Push(target string, opts *http.PushOptions) error {
	pusher, ok := rw.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}
