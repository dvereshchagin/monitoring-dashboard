package port

import (
	"context"

	"github.com/dreschagin/monitoring-dashboard/internal/domain/valueobject"
)

// RawMetric представляет сырую метрику от collector'а
// Используется для передачи данных между Infrastructure и Application слоями
type RawMetric struct {
	Type     valueobject.MetricType
	Name     string
	Value    valueobject.MetricValue
	Metadata map[string]interface{}
}

// MetricsCollector определяет интерфейс для сбора метрик (Port)
// Реализация будет в Infrastructure слое
type MetricsCollector interface {
	// CollectAll собирает все доступные метрики
	CollectAll(ctx context.Context) ([]RawMetric, error)

	// CollectCPU собирает метрики CPU
	CollectCPU(ctx context.Context) ([]RawMetric, error)

	// CollectMemory собирает метрики памяти
	CollectMemory(ctx context.Context) ([]RawMetric, error)

	// CollectDisk собирает метрики дисков
	CollectDisk(ctx context.Context) ([]RawMetric, error)

	// CollectNetwork собирает метрики сети
	CollectNetwork(ctx context.Context) ([]RawMetric, error)
}
