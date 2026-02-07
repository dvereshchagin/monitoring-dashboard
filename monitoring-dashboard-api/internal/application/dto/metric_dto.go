package dto

import (
	"time"

	"github.com/dreschagin/monitoring-dashboard/internal/domain/entity"
)

// MetricDTO представляет метрику для передачи между слоями
type MetricDTO struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Value       float64                `json:"value"`
	Unit        string                 `json:"unit"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CollectedAt time.Time              `json:"collected_at"`
	CreatedAt   time.Time              `json:"created_at"`
	// Computed fields
	IsCritical bool `json:"is_critical"`
	IsWarning  bool `json:"is_warning"`
}

// FromEntity конвертирует Domain Entity в DTO
func FromEntity(metric *entity.Metric) *MetricDTO {
	return &MetricDTO{
		ID:          metric.ID(),
		Type:        metric.Type().String(),
		Name:        metric.Name(),
		Value:       metric.Value().Raw(),
		Unit:        metric.Value().Unit(),
		Metadata:    metric.Metadata(),
		CollectedAt: metric.CollectedAt(),
		CreatedAt:   metric.CreatedAt(),
		IsCritical:  metric.IsCritical(),
		IsWarning:   metric.IsWarning(),
	}
}

// ToMetricDTOs конвертирует слайс Entity в слайс DTO
func ToMetricDTOs(metrics []*entity.Metric) []*MetricDTO {
	dtos := make([]*MetricDTO, len(metrics))
	for i, m := range metrics {
		dtos[i] = FromEntity(m)
	}
	return dtos
}
