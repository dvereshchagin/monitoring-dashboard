package usecase

import (
	"context"
	"fmt"

	"github.com/dreschagin/monitoring-dashboard/internal/application/dto"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/repository"
	"github.com/dreschagin/monitoring-dashboard/pkg/logger"
)

// GetCurrentMetricsUseCase возвращает текущие метрики (последние по каждому типу)
type GetCurrentMetricsUseCase struct {
	repository repository.MetricRepository
	logger     *logger.Logger
}

// NewGetCurrentMetricsUseCase создает новый use case
func NewGetCurrentMetricsUseCase(
	repository repository.MetricRepository,
	logger *logger.Logger,
) *GetCurrentMetricsUseCase {
	return &GetCurrentMetricsUseCase{
		repository: repository,
		logger:     logger,
	}
}

// Execute выполняет получение текущих метрик
func (uc *GetCurrentMetricsUseCase) Execute(ctx context.Context) (*dto.MetricSnapshotDTO, error) {
	uc.logger.Debug("Fetching current metrics")

	// Получаем последние метрики каждого типа
	latestMetrics, err := uc.repository.FindLatest(ctx)
	if err != nil {
		uc.logger.Error("Failed to fetch latest metrics", err)
		return nil, fmt.Errorf("failed to fetch latest metrics: %w", err)
	}

	uc.logger.Debug("Fetched latest metrics", "count", len(latestMetrics))

	// Конвертируем в DTO и создаем snapshot
	snapshot := dto.NewMetricSnapshotDTO(latestMetrics)

	return snapshot, nil
}
