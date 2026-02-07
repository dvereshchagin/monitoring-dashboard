package collector

import (
	"context"
	"time"

	"github.com/dreschagin/monitoring-dashboard/internal/application/port"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/valueobject"
	"github.com/shirou/gopsutil/v3/net"
)

// NetworkCollector собирает метрики сети
type NetworkCollector struct {
	lastStats     map[string]net.IOCountersStat
	lastCheckTime time.Time
}

// NewNetworkCollector создает новый Network collector
func NewNetworkCollector() *NetworkCollector {
	return &NetworkCollector{
		lastStats:     make(map[string]net.IOCountersStat),
		lastCheckTime: time.Now(),
	}
}

// Collect собирает Network метрики
func (c *NetworkCollector) Collect(ctx context.Context) ([]port.RawMetric, error) {
	stats, err := net.IOCountersWithContext(ctx, false)
	if err != nil || len(stats) == 0 {
		return nil, err
	}

	currentTime := time.Now()
	currentStats := stats[0]

	var metrics []port.RawMetric

	// Если есть предыдущие данные, вычисляем скорость
	if lastStat, exists := c.lastStats["all"]; exists {
		duration := currentTime.Sub(c.lastCheckTime).Seconds()
		if duration > 0 {
			// Вычисляем скорость в KB/s
			bytesSentPerSec := float64(currentStats.BytesSent-lastStat.BytesSent) / duration / 1024
			bytesRecvPerSec := float64(currentStats.BytesRecv-lastStat.BytesRecv) / duration / 1024

			// Метрика для отправленных данных
			valueSent, _ := valueobject.NewMetricValue(bytesSentPerSec, "KB/s")
			metrics = append(metrics, port.RawMetric{
				Type:  valueobject.Network,
				Name:  "network_sent",
				Value: valueSent,
				Metadata: map[string]interface{}{
					"interface":    "all",
					"packets_sent": currentStats.PacketsSent,
				},
			})

			// Для упрощения, используем только sent rate как основную метрику
			// В production можно добавить отдельные метрики для sent/recv
			_ = bytesRecvPerSec
		}
	}

	// Сохраняем текущие данные для следующего вызова
	c.lastStats["all"] = currentStats
	c.lastCheckTime = currentTime

	return metrics, nil
}
