package repository

import (
	"context"

	"github.com/dreschagin/monitoring-dashboard/internal/domain/entity"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/valueobject"
)

// MetricRepository определяет интерфейс для работы с хранилищем метрик (Port)
// Реализация будет в Infrastructure слое
type MetricRepository interface {
	// Save сохраняет одну метрику
	Save(ctx context.Context, metric *entity.Metric) error

	// SaveBatch сохраняет несколько метрик одной транзакцией
	SaveBatch(ctx context.Context, metrics []*entity.Metric) error

	// FindByID находит метрику по идентификатору
	FindByID(ctx context.Context, id string) (*entity.Metric, error)

	// FindByType находит метрики по типу с ограничением количества
	FindByType(ctx context.Context, metricType valueobject.MetricType, limit int) ([]*entity.Metric, error)

	// FindByTimeRange находит метрики по типу и временному диапазону
	FindByTimeRange(
		ctx context.Context,
		metricType valueobject.MetricType,
		timeRange valueobject.TimeRange,
	) ([]*entity.Metric, error)

	// FindLatest находит последние метрики каждого типа
	FindLatest(ctx context.Context) (map[valueobject.MetricType]*entity.Metric, error)

	// FindLatestByType находит последнюю метрику указанного типа
	FindLatestByType(ctx context.Context, metricType valueobject.MetricType) (*entity.Metric, error)

	// DeleteOlderThan удаляет метрики старше указанного времени
	DeleteOlderThan(ctx context.Context, age valueobject.TimeRange) error

	// Count возвращает количество метрик по типу
	Count(ctx context.Context, metricType valueobject.MetricType) (int64, error)
}
