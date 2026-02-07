package collector

import (
	"context"
	"time"

	"github.com/dreschagin/monitoring-dashboard/internal/application/port"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/valueobject"
	"github.com/shirou/gopsutil/v3/cpu"
)

// CPUCollector собирает метрики CPU
type CPUCollector struct{}

// NewCPUCollector создает новый CPU collector
func NewCPUCollector() *CPUCollector {
	return &CPUCollector{}
}

// Collect собирает CPU метрики
func (c *CPUCollector) Collect(ctx context.Context) ([]port.RawMetric, error) {
	// Получаем процент использования CPU за 1 секунду
	percentages, err := cpu.PercentWithContext(ctx, time.Second, false)
	if err != nil {
		return nil, err
	}

	// Получаем количество ядер
	counts, _ := cpu.Counts(true)

	var metrics []port.RawMetric

	// Создаем метрику для общего использования CPU
	if len(percentages) > 0 {
		value, _ := valueobject.NewMetricValue(percentages[0], "%")
		metrics = append(metrics, port.RawMetric{
			Type:  valueobject.CPU,
			Name:  "cpu_usage",
			Value: value,
			Metadata: map[string]interface{}{
				"cores": counts,
			},
		})
	}

	return metrics, nil
}
