package collector

import (
	"context"

	"github.com/dreschagin/monitoring-dashboard/internal/application/port"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/valueobject"
	"github.com/shirou/gopsutil/v3/mem"
)

// MemoryCollector собирает метрики памяти
type MemoryCollector struct{}

// NewMemoryCollector создает новый Memory collector
func NewMemoryCollector() *MemoryCollector {
	return &MemoryCollector{}
}

// Collect собирает Memory метрики
func (c *MemoryCollector) Collect(ctx context.Context) ([]port.RawMetric, error) {
	vmStat, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var metrics []port.RawMetric

	// Процент использования памяти
	value, _ := valueobject.NewMetricValue(vmStat.UsedPercent, "%")
	metrics = append(metrics, port.RawMetric{
		Type:  valueobject.Memory,
		Name:  "memory_usage",
		Value: value,
		Metadata: map[string]interface{}{
			"total_mb": vmStat.Total / 1024 / 1024,
			"used_mb":  vmStat.Used / 1024 / 1024,
			"free_mb":  vmStat.Free / 1024 / 1024,
		},
	})

	return metrics, nil
}
