package collector

import (
	"context"
	"sync"

	"github.com/dreschagin/monitoring-dashboard/internal/application/port"
)

// SystemMetricsCollector собирает все системные метрики
// Реализует интерфейс port.MetricsCollector
type SystemMetricsCollector struct {
	cpuCollector     *CPUCollector
	memoryCollector  *MemoryCollector
	diskCollector    *DiskCollector
	networkCollector *NetworkCollector
}

// NewSystemMetricsCollector создает новый системный collector
func NewSystemMetricsCollector() *SystemMetricsCollector {
	return &SystemMetricsCollector{
		cpuCollector:     NewCPUCollector(),
		memoryCollector:  NewMemoryCollector(),
		diskCollector:    NewDiskCollector(),
		networkCollector: NewNetworkCollector(),
	}
}

// CollectAll собирает все доступные метрики параллельно
func (c *SystemMetricsCollector) CollectAll(ctx context.Context) ([]port.RawMetric, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	allMetrics := make([]port.RawMetric, 0)

	// Функция для сбора метрик с обработкой ошибок
	collectFunc := func(collector func(context.Context) ([]port.RawMetric, error)) {
		defer wg.Done()
		metrics, err := collector(ctx)
		if err != nil {
			// Логируем ошибку, но продолжаем
			return
		}
		mu.Lock()
		allMetrics = append(allMetrics, metrics...)
		mu.Unlock()
	}

	// Запускаем сбор всех метрик параллельно
	wg.Add(4)
	go collectFunc(c.cpuCollector.Collect)
	go collectFunc(c.memoryCollector.Collect)
	go collectFunc(c.diskCollector.Collect)
	go collectFunc(c.networkCollector.Collect)

	wg.Wait()

	return allMetrics, nil
}

// CollectCPU собирает только CPU метрики
func (c *SystemMetricsCollector) CollectCPU(ctx context.Context) ([]port.RawMetric, error) {
	return c.cpuCollector.Collect(ctx)
}

// CollectMemory собирает только Memory метрики
func (c *SystemMetricsCollector) CollectMemory(ctx context.Context) ([]port.RawMetric, error) {
	return c.memoryCollector.Collect(ctx)
}

// CollectDisk собирает только Disk метрики
func (c *SystemMetricsCollector) CollectDisk(ctx context.Context) ([]port.RawMetric, error) {
	return c.diskCollector.Collect(ctx)
}

// CollectNetwork собирает только Network метрики
func (c *SystemMetricsCollector) CollectNetwork(ctx context.Context) ([]port.RawMetric, error) {
	return c.networkCollector.Collect(ctx)
}
