package http

import (
	"io/fs"
	"net/http"

	"github.com/dreschagin/monitoring-dashboard/internal/interfaces/http/handler"
	"github.com/dreschagin/monitoring-dashboard/internal/interfaces/http/middleware"
	"github.com/dreschagin/monitoring-dashboard/pkg/config"
	"github.com/dreschagin/monitoring-dashboard/pkg/logger"
)

// Router настраивает маршруты приложения
type Router struct {
	mux                       *http.ServeMux
	dashboardHandler          *handler.DashboardHandler
	websocketHandler          *handler.WebSocketHandler
	metricsAPIHandler         *handler.MetricsAPIHandler
	screenshotAPIHandler      *handler.ScreenshotAPIHandler
	authAPIHandler            *handler.AuthAPIHandler
	releaseAnalyzerAPIHandler *handler.ReleaseAnalyzerAPIHandler
	security                  config.SecurityConfig
	logger                    *logger.Logger
}

// NewRouter создает новый router
func NewRouter(
	dashboardHandler *handler.DashboardHandler,
	websocketHandler *handler.WebSocketHandler,
	metricsAPIHandler *handler.MetricsAPIHandler,
	screenshotAPIHandler *handler.ScreenshotAPIHandler,
	authAPIHandler *handler.AuthAPIHandler,
	releaseAnalyzerAPIHandler *handler.ReleaseAnalyzerAPIHandler,
	security config.SecurityConfig,
	logger *logger.Logger,
) *Router {
	return &Router{
		mux:                       http.NewServeMux(),
		dashboardHandler:          dashboardHandler,
		websocketHandler:          websocketHandler,
		metricsAPIHandler:         metricsAPIHandler,
		screenshotAPIHandler:      screenshotAPIHandler,
		authAPIHandler:            authAPIHandler,
		releaseAnalyzerAPIHandler: releaseAnalyzerAPIHandler,
		security:                  security,
		logger:                    logger,
	}
}

// Setup настраивает все маршруты
func (rt *Router) Setup() http.Handler {
	// Static assets are embedded into the binary.
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("failed to initialize embedded static assets: " + err.Error())
	}
	rt.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServerFS(staticFS)))

	// Health endpoints are intentionally unauthenticated for probes.
	rt.mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	rt.mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	authMiddleware := middleware.Auth(middleware.AuthConfig{
		Enabled:     rt.security.AuthEnabled,
		BearerToken: rt.security.AuthToken,
	}, rt.logger)

	// Dashboard
	rt.mux.Handle("/", authMiddleware(http.HandlerFunc(rt.dashboardHandler.ShowDashboard)))

	// WebSocket
	rt.mux.Handle("/ws", authMiddleware(http.HandlerFunc(rt.websocketHandler.HandleConnection)))

	// API endpoints
	rt.mux.HandleFunc("/api/v1/auth/login", rt.authAPIHandler.Login)
	rt.mux.HandleFunc("/api/v1/auth/logout", rt.authAPIHandler.Logout)
	rt.mux.HandleFunc("/api/v1/auth/status", rt.authAPIHandler.Status)

	rt.mux.Handle("/api/v1/metrics/history", authMiddleware(http.HandlerFunc(rt.metricsAPIHandler.GetHistoricalMetrics)))
	rt.mux.Handle("/api/metrics/history", authMiddleware(http.HandlerFunc(rt.metricsAPIHandler.GetHistoricalMetrics)))
	rt.mux.Handle("/api/v1/screenshots/dashboard", authMiddleware(http.HandlerFunc(rt.screenshotAPIHandler.HandleDashboardScreenshots)))
	rt.mux.Handle("/api/v1/release-analyzer/summary", authMiddleware(http.HandlerFunc(rt.releaseAnalyzerAPIHandler.GetSummary)))
	rt.mux.Handle("/api/v1/release-analyzer/run", authMiddleware(http.HandlerFunc(rt.releaseAnalyzerAPIHandler.RunNow)))

	// Применяем middleware
	var handler http.Handler = rt.mux
	handler = middleware.Logger(rt.logger)(handler)
	handler = middleware.Recovery(rt.logger)(handler)

	return handler
}
