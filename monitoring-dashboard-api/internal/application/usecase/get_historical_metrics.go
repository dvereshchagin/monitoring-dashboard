package usecase

import (
	"context"
	"fmt"

	"github.com/dreschagin/monitoring-dashboard/internal/application/dto"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/repository"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/service"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/valueobject"
	"github.com/dreschagin/monitoring-dashboard/pkg/logger"
)

// GetHistoricalMetricsUseCase возвращает исторические метрики за указанный период
type GetHistoricalMetricsUseCase struct {
	repository repository.MetricRepository
	aggregator *service.MetricAggregator
	logger     *logger.Logger
}

// NewGetHistoricalMetricsUseCase создает новый use case
func NewGetHistoricalMetricsUseCase(
	repository repository.MetricRepository,
	aggregator *service.MetricAggregator,
	logger *logger.Logger,
) *GetHistoricalMetricsUseCase {
	return &GetHistoricalMetricsUseCase{
		repository: repository,
		aggregator: aggregator,
		logger:     logger,
	}
}

// Execute выполняет получение исторических метрик
func (uc *GetHistoricalMetricsUseCase) Execute(
	ctx context.Context,
	metricType valueobject.MetricType,
	timeRange valueobject.TimeRange,
) ([]*dto.MetricDTO, error) {
	uc.logger.Debug("Fetching historical metrics",
		"type", metricType.String(),
		"start", timeRange.Start(),
		"end", timeRange.End())

	// Валидация типа метрики
	if err := metricType.Validate(); err != nil {
		return nil, fmt.Errorf("invalid metric type: %w", err)
	}

	// Получаем метрики из репозитория
	metrics, err := uc.repository.FindByTimeRange(ctx, metricType, timeRange)
	if err != nil {
		uc.logger.Error("Failed to fetch historical metrics", err)
		return nil, fmt.Errorf("failed to fetch historical metrics: %w", err)
	}

	uc.logger.Debug("Fetched historical metrics", "count", len(metrics))

	// Сортируем по времени (по возрастанию для графиков)
	sortedMetrics := uc.aggregator.SortByTime(metrics, false)

	// Конвертируем в DTOs
	dtos := dto.ToMetricDTOs(sortedMetrics)

	return dtos, nil
}

// ExecuteWithAggregation возвращает исторические метрики с агрегированными данными
func (uc *GetHistoricalMetricsUseCase) ExecuteWithAggregation(
	ctx context.Context,
	metricType valueobject.MetricType,
	timeRange valueobject.TimeRange,
) (*dto.MetricHistoryDTO, error) {
	// Получаем метрики
	metrics, err := uc.repository.FindByTimeRange(ctx, metricType, timeRange)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch historical metrics: %w", err)
	}

	if len(metrics) == 0 {
		return &dto.MetricHistoryDTO{
			Type:    metricType.String(),
			Metrics: []*dto.MetricDTO{},
		}, nil
	}

	// Вычисляем агрегаты
	avg, _ := uc.aggregator.CalculateAverage(metrics)
	min, _ := uc.aggregator.CalculateMin(metrics)
	max, _ := uc.aggregator.CalculateMax(metrics)

	// Находим критические и предупреждающие метрики
	critical := uc.aggregator.FindCritical(metrics)
	warnings := uc.aggregator.FindWarning(metrics)

	// Сортируем по времени
	sortedMetrics := uc.aggregator.SortByTime(metrics, false)

	return &dto.MetricHistoryDTO{
		Type:          metricType.String(),
		Metrics:       dto.ToMetricDTOs(sortedMetrics),
		Average:       avg,
		Min:           min,
		Max:           max,
		CriticalCount: len(critical),
		WarningCount:  len(warnings),
	}, nil
}
