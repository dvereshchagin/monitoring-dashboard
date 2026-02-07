package collector

import (
	"context"

	"github.com/dreschagin/monitoring-dashboard/internal/application/port"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/valueobject"
	"github.com/shirou/gopsutil/v3/disk"
)

// DiskCollector собирает метрики дисков
type DiskCollector struct{}

// NewDiskCollector создает новый Disk collector
func NewDiskCollector() *DiskCollector {
	return &DiskCollector{}
}

// Collect собирает Disk метрики
func (c *DiskCollector) Collect(ctx context.Context) ([]port.RawMetric, error) {
	// Получаем информацию о корневом разделе
	usage, err := disk.UsageWithContext(ctx, "/")
	if err != nil {
		return nil, err
	}

	var metrics []port.RawMetric

	// Процент использования диска
	value, _ := valueobject.NewMetricValue(usage.UsedPercent, "%")
	metrics = append(metrics, port.RawMetric{
		Type:  valueobject.Disk,
		Name:  "disk_usage",
		Value: value,
		Metadata: map[string]interface{}{
			"mount":    usage.Path,
			"total_gb": usage.Total / 1024 / 1024 / 1024,
			"used_gb":  usage.Used / 1024 / 1024 / 1024,
			"free_gb":  usage.Free / 1024 / 1024 / 1024,
		},
	})

	return metrics, nil
}
