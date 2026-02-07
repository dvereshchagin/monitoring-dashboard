package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dreschagin/monitoring-dashboard/internal/application/usecase"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/valueobject"
	"github.com/dreschagin/monitoring-dashboard/pkg/logger"
)

// MetricsAPIHandler обрабатывает API запросы для метрик
type MetricsAPIHandler struct {
	getHistoricalMetricsUC *usecase.GetHistoricalMetricsUseCase
	maxDuration            time.Duration
	logger                 *logger.Logger
}

// NewMetricsAPIHandler создает новый handler
func NewMetricsAPIHandler(
	getHistoricalMetricsUC *usecase.GetHistoricalMetricsUseCase,
	maxDuration time.Duration,
	logger *logger.Logger,
) *MetricsAPIHandler {
	if maxDuration <= 0 {
		maxDuration = 24 * time.Hour
	}

	return &MetricsAPIHandler{
		getHistoricalMetricsUC: getHistoricalMetricsUC,
		maxDuration:            maxDuration,
		logger:                 logger,
	}
}

// GetHistoricalMetrics возвращает исторические данные
func (h *MetricsAPIHandler) GetHistoricalMetrics(w http.ResponseWriter, r *http.Request) {
	// Получаем параметры из query string
	metricTypeStr := r.URL.Query().Get("type")
	durationStr := r.URL.Query().Get("duration")

	if metricTypeStr == "" || durationStr == "" {
		http.Error(w, "Missing required parameters: type, duration", http.StatusBadRequest)
		return
	}

	// Парсим metric type
	metricType := valueobject.MetricType(metricTypeStr)
	if err := metricType.Validate(); err != nil {
		http.Error(w, "Invalid metric type", http.StatusBadRequest)
		return
	}

	// Парсим duration
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		http.Error(w, "Invalid duration format", http.StatusBadRequest)
		return
	}
	if duration <= 0 || duration > h.maxDuration {
		http.Error(w, "Duration out of allowed range", http.StatusBadRequest)
		return
	}

	// Создаем time range
	timeRange, err := valueobject.NewTimeRangeFromDuration(duration)
	if err != nil {
		http.Error(w, "Invalid time range", http.StatusBadRequest)
		return
	}

	// Получаем метрики
	history, err := h.getHistoricalMetricsUC.ExecuteWithAggregation(r.Context(), metricType, timeRange)
	if err != nil {
		h.logger.Error("Failed to get historical metrics", err)
		http.Error(w, "Failed to fetch metrics", http.StatusInternalServerError)
		return
	}

	// Отправляем JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(history); err != nil {
		h.logger.Error("Failed to encode historical metrics response", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
