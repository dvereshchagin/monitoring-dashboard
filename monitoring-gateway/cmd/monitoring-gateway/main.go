package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/dreschagin/monitoring-dashboard/monitoring-gateway/internal/auth"
	"github.com/dreschagin/monitoring-dashboard/monitoring-gateway/internal/discovery"
	k8sdiscovery "github.com/dreschagin/monitoring-dashboard/monitoring-gateway/internal/discovery/k8s"
	"github.com/dreschagin/monitoring-dashboard/monitoring-gateway/internal/httpx"
	gatewaymetrics "github.com/dreschagin/monitoring-dashboard/monitoring-gateway/internal/metrics"
	"github.com/dreschagin/monitoring-dashboard/monitoring-gateway/internal/proxy"
	"github.com/dreschagin/monitoring-dashboard/monitoring-gateway/internal/ratelimit"
	"github.com/dreschagin/monitoring-dashboard/monitoring-gateway/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.LogLevel)

	resolver, err := buildResolver(cfg)
	if err != nil {
		logger.Error("failed to initialize service discovery", "error", err)
		os.Exit(1)
	}

	discoveryManager := discovery.NewManager(resolver, cfg.Discovery.RefreshInterval)

	initialCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	metricsRegistry := prometheus.NewRegistry()
	metrics := gatewaymetrics.New(metricsRegistry)
	metrics.DiscoveryRefreshes.Inc()
	if err := discoveryManager.Refresh(initialCtx); err != nil {
		metrics.DiscoveryErrors.Inc()
		logger.Error("initial discovery failed", "error", err)
	} else {
		logger.Info("initial discovery completed")
	}
	cancel()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		ticker := time.NewTicker(cfg.Discovery.RefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				metrics.DiscoveryRefreshes.Inc()
				refreshCtx, refreshCancel := context.WithTimeout(ctx, 10*time.Second)
				err := discoveryManager.Refresh(refreshCtx)
				refreshCancel()
				if err != nil {
					metrics.DiscoveryErrors.Inc()
					logger.Error("discovery refresh failed", "error", err)
				}
			}
		}
	}()

	proxyHandler := proxy.NewHandler(discoveryManager, cfg.Upstream.Timeout, logger, metrics)
	limiter := ratelimit.New(cfg.RateLimit.RPS, cfg.RateLimit.Burst)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !discoveryManager.Ready() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})
	mux.Handle("/metrics", promhttp.HandlerFor(metricsRegistry, promhttp.HandlerOpts{}))

	var apiHandler http.Handler = proxyHandler
	apiHandler = auth.Middleware(cfg.Auth.Enabled, cfg.Auth.BearerToken, metrics, apiHandler)
	apiHandler = limiter.Middleware(metrics, apiHandler)
	apiHandler = metrics.Middleware(apiHandler)
	apiHandler = httpx.WithRequestID(apiHandler)
	apiHandler = httpx.WithLogging(logger, apiHandler)

	mux.Handle("/", apiHandler)

	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("gateway server started", "port", cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("gateway server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("failed to shutdown gateway server", "error", err)
	}
}

func buildResolver(cfg *config.Config) (discovery.Resolver, error) {
	if cfg.Discovery.Enabled {
		return k8sdiscovery.NewInClusterResolver(
			cfg.Discovery.Namespace,
			cfg.Discovery.APIServiceSelector,
			cfg.Discovery.AnalyzerServiceSelector,
			cfg.Discovery.AnalyzerRequired,
		)
	}

	return discovery.NewStaticResolver(cfg.Upstream.APIURL, cfg.Upstream.AnalyzerURL)
}

func newLogger(level string) *slog.Logger {
	var slogLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel})
	return slog.New(handler)
}
