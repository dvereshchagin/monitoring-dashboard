package dto

import (
	"time"

	"github.com/dreschagin/monitoring-dashboard/internal/domain/entity"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/valueobject"
)

// MetricSnapshotDTO представляет snapshot всех метрик
// Используется для передачи через WebSocket
type MetricSnapshotDTO struct {
	Timestamp time.Time           `json:"timestamp"`
	CPU       *MetricDTO          `json:"cpu,omitempty"`
	Memory    *MetricDTO          `json:"memory,omitempty"`
	Disk      *MetricDTO          `json:"disk,omitempty"`
	Network   *MetricDTO          `json:"network,omitempty"`
	Summary   *SnapshotSummaryDTO `json:"summary"`
}

// SnapshotSummaryDTO содержит сводную информацию
type SnapshotSummaryDTO struct {
	TotalMetrics  int    `json:"total_metrics"`
	CriticalCount int    `json:"critical_count"`
	WarningCount  int    `json:"warning_count"`
	HasCritical   bool   `json:"has_critical"`
	HasWarning    bool   `json:"has_warning"`
	OverallStatus string `json:"overall_status"` // "healthy", "warning", "critical"
}

// NewMetricSnapshotDTO создает snapshot из map метрик
func NewMetricSnapshotDTO(metricsMap map[valueobject.MetricType]*entity.Metric) *MetricSnapshotDTO {
	snapshot := &MetricSnapshotDTO{
		Timestamp: time.Now(),
		Summary:   &SnapshotSummaryDTO{},
	}

	var criticalCount, warningCount int

	// Конвертируем каждую метрику
	for metricType, metric := range metricsMap {
		if metric == nil {
			continue
		}

		dto := FromEntity(metric)
		snapshot.Summary.TotalMetrics++

		if dto.IsCritical {
			criticalCount++
		} else if dto.IsWarning {
			warningCount++
		}

		// Распределяем по типам
		switch metricType {
		case valueobject.CPU:
			snapshot.CPU = dto
		case valueobject.Memory:
			snapshot.Memory = dto
		case valueobject.Disk:
			snapshot.Disk = dto
		case valueobject.Network:
			snapshot.Network = dto
		}
	}

	// Заполняем summary
	snapshot.Summary.CriticalCount = criticalCount
	snapshot.Summary.WarningCount = warningCount
	snapshot.Summary.HasCritical = criticalCount > 0
	snapshot.Summary.HasWarning = warningCount > 0

	// Определяем общий статус
	if criticalCount > 0 {
		snapshot.Summary.OverallStatus = "critical"
	} else if warningCount > 0 {
		snapshot.Summary.OverallStatus = "warning"
	} else {
		snapshot.Summary.OverallStatus = "healthy"
	}

	return snapshot
}

// NewMetricSnapshotFromSlice создает snapshot из слайса метрик
func NewMetricSnapshotFromSlice(metrics []*entity.Metric) *MetricSnapshotDTO {
	metricsMap := make(map[valueobject.MetricType]*entity.Metric)

	for _, metric := range metrics {
		metricsMap[metric.Type()] = metric
	}

	return NewMetricSnapshotDTO(metricsMap)
}

// AlertDTO представляет alert для отправки клиентам
type AlertDTO struct {
	Timestamp time.Time  `json:"timestamp"`
	Level     string     `json:"level"` // "warning", "critical"
	Metric    *MetricDTO `json:"metric"`
	Message   string     `json:"message"`
}

// NewAlertDTO создает новый alert
func NewAlertDTO(metric *entity.Metric, message string) *AlertDTO {
	level := "warning"
	if metric.IsCritical() {
		level = "critical"
	}

	return &AlertDTO{
		Timestamp: time.Now(),
		Level:     level,
		Metric:    FromEntity(metric),
		Message:   message,
	}
}

// MetricHistoryDTO представляет исторические данные метрик с агрегатами
type MetricHistoryDTO struct {
	Type          string       `json:"type"`
	Metrics       []*MetricDTO `json:"metrics"`
	Average       float64      `json:"average"`
	Min           float64      `json:"min"`
	Max           float64      `json:"max"`
	CriticalCount int          `json:"critical_count"`
	WarningCount  int          `json:"warning_count"`
}
