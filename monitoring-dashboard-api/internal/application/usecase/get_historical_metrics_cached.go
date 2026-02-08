package usecase

import (
	"context"
	"fmt"

	"github.com/dreschagin/monitoring-dashboard/internal/application/dto"
	"github.com/dreschagin/monitoring-dashboard/internal/application/port"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/repository"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/service"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/valueobject"
	"github.com/dreschagin/monitoring-dashboard/internal/infrastructure/cache/redis"
	"github.com/dreschagin/monitoring-dashboard/pkg/logger"
)

// GetHistoricalMetricsCachedUseCase возвращает исторические метрики с кешированием
type GetHistoricalMetricsCachedUseCase struct {
	repository repository.MetricRepository
	aggregator *service.MetricAggregator
	cache      port.Cache
	logger     *logger.Logger
}

// NewGetHistoricalMetricsCachedUseCase создает новый use case с кешированием
func NewGetHistoricalMetricsCachedUseCase(
	repository repository.MetricRepository,
	aggregator *service.MetricAggregator,
	cache port.Cache,
	logger *logger.Logger,
) *GetHistoricalMetricsCachedUseCase {
	return &GetHistoricalMetricsCachedUseCase{
		repository: repository,
		aggregator: aggregator,
		cache:      cache,
		logger:     logger,
	}
}

// Execute выполняет получение исторических метрик с кешированием
func (uc *GetHistoricalMetricsCachedUseCase) Execute(
	ctx context.Context,
	metricType valueobject.MetricType,
	timeRange valueobject.TimeRange,
) ([]*dto.MetricDTO, error) {
	// Валидация типа метрики
	if err := metricType.Validate(); err != nil {
		return nil, fmt.Errorf("invalid metric type: %w", err)
	}

	// Если кеш не настроен, используем стандартный путь
	if uc.cache == nil {
		return uc.executeWithoutCache(ctx, metricType, timeRange)
	}

	// Генерируем ключ кеша
	duration := timeRange.End().Sub(timeRange.Start()).String()
	cacheKey := redis.GenerateCacheKey(metricType.String(), duration)

	// Пытаемся получить из кеша
	var cachedDTOs []*dto.MetricDTO
	err := uc.cache.Get(ctx, cacheKey, &cachedDTOs)
	if err == nil {
		uc.logger.Debug("Cache hit for historical metrics",
			"type", metricType.String(),
			"count", len(cachedDTOs))
		return cachedDTOs, nil
	}

	// Cache miss - получаем из БД
	uc.logger.Debug("Cache miss for historical metrics, fetching from DB",
		"type", metricType.String())

	dtos, err := uc.executeWithoutCache(ctx, metricType, timeRange)
	if err != nil {
		return nil, err
	}

	// Сохраняем в кеш (асинхронно, не блокируем ответ)
	go func() {
		if err := uc.cache.Set(context.Background(), cacheKey, dtos); err != nil {
			uc.logger.Warn("Failed to cache metrics", err)
		}
	}()

	return dtos, nil
}

// executeWithoutCache получает метрики без кеширования
func (uc *GetHistoricalMetricsCachedUseCase) executeWithoutCache(
	ctx context.Context,
	metricType valueobject.MetricType,
	timeRange valueobject.TimeRange,
) ([]*dto.MetricDTO, error) {
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

// ExecuteWithAggregation возвращает исторические метрики с агрегированными данными и кешированием
func (uc *GetHistoricalMetricsCachedUseCase) ExecuteWithAggregation(
	ctx context.Context,
	metricType valueobject.MetricType,
	timeRange valueobject.TimeRange,
) (*dto.MetricHistoryDTO, error) {
	// Валидация типа метрики
	if err := metricType.Validate(); err != nil {
		return nil, fmt.Errorf("invalid metric type: %w", err)
	}

	// Если кеш не настроен, используем стандартный путь
	if uc.cache == nil {
		return uc.executeAggregationWithoutCache(ctx, metricType, timeRange)
	}

	// Генерируем ключ кеша с префиксом для агрегированных данных
	duration := timeRange.End().Sub(timeRange.Start()).String()
	cacheKey := fmt.Sprintf("metrics:history:agg:%s:%s", metricType.String(), duration)

	// Пытаемся получить из кеша
	var cachedHistory *dto.MetricHistoryDTO
	err := uc.cache.Get(ctx, cacheKey, &cachedHistory)
	if err == nil {
		uc.logger.Debug("Cache hit for aggregated historical metrics",
			"type", metricType.String())
		return cachedHistory, nil
	}

	// Cache miss - получаем из БД
	history, err := uc.executeAggregationWithoutCache(ctx, metricType, timeRange)
	if err != nil {
		return nil, err
	}

	// Сохраняем в кеш (асинхронно)
	go func() {
		if err := uc.cache.Set(context.Background(), cacheKey, history); err != nil {
			uc.logger.Warn("Failed to cache aggregated metrics", err)
		}
	}()

	return history, nil
}

// executeAggregationWithoutCache получает агрегированные метрики без кеширования
func (uc *GetHistoricalMetricsCachedUseCase) executeAggregationWithoutCache(
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
