package handler

import (
	"net/http"

	"github.com/dreschagin/monitoring-dashboard/internal/application/usecase"
	"github.com/dreschagin/monitoring-dashboard/internal/interfaces/view"
	"github.com/dreschagin/monitoring-dashboard/pkg/logger"
)

// DashboardHandler обрабатывает запросы к dashboard
type DashboardHandler struct {
	getCurrentMetricsUC *usecase.GetCurrentMetricsUseCase
	logger              *logger.Logger
}

// NewDashboardHandler создает новый handler
func NewDashboardHandler(
	getCurrentMetricsUC *usecase.GetCurrentMetricsUseCase,
	logger *logger.Logger,
) *DashboardHandler {
	return &DashboardHandler{
		getCurrentMetricsUC: getCurrentMetricsUC,
		logger:              logger,
	}
}

// ShowDashboard отображает главную страницу dashboard
func (h *DashboardHandler) ShowDashboard(w http.ResponseWriter, r *http.Request) {
	// Получаем текущие метрики
	snapshot, err := h.getCurrentMetricsUC.Execute(r.Context())
	if err != nil {
		h.logger.Error("Failed to get current metrics", err)
		http.Error(w, "Failed to load metrics", http.StatusInternalServerError)
		return
	}

	// Рендерим Templ template
	if err := view.Dashboard(snapshot).Render(r.Context(), w); err != nil {
		h.logger.Error("Failed to render dashboard", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
		return
	}
}
