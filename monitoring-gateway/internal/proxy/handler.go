package proxy

import (
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/dreschagin/monitoring-dashboard/monitoring-gateway/internal/discovery"
	gatewaymetrics "github.com/dreschagin/monitoring-dashboard/monitoring-gateway/internal/metrics"
	"github.com/dreschagin/monitoring-dashboard/monitoring-gateway/internal/routing"
)

// Handler proxies client requests to discovered upstream services.
type Handler struct {
	discovery *discovery.Manager
	timeout   time.Duration
	logger    *slog.Logger
	metrics   *gatewaymetrics.Metrics
}

func NewHandler(discoveryManager *discovery.Manager, timeout time.Duration, logger *slog.Logger, metrics *gatewaymetrics.Metrics) *Handler {
	return &Handler{
		discovery: discoveryManager,
		timeout:   timeout,
		logger:    logger,
		metrics:   metrics,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	target, ok := routing.Match(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	snapshot, ready := h.discovery.Snapshot()
	if !ready {
		http.Error(w, "gateway is not ready", http.StatusServiceUnavailable)
		return
	}

	upstream := h.resolveUpstream(snapshot, target)
	if upstream == nil {
		http.Error(w, "upstream is unavailable", http.StatusServiceUnavailable)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.Transport = &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: h.timeout, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   64,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: h.timeout,
	}

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = upstream.Host
	}

	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		h.metrics.UpstreamErrors.Inc()
		h.logger.Error("proxy request failed",
			"error", err,
			"path", req.URL.Path,
			"upstream", upstream.String(),
		)
		http.Error(rw, "bad gateway", http.StatusBadGateway)
	}

	proxy.ServeHTTP(w, r)
}

func (h *Handler) resolveUpstream(snapshot discovery.Snapshot, target routing.Target) *url.URL {
	switch target {
	case routing.TargetAPI:
		return snapshot.APIURL
	case routing.TargetAnalyzer:
		return snapshot.AnalyzerURL
	default:
		return nil
	}
}
