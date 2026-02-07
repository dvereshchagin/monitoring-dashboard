package service

import (
	"errors"
	"sort"

	"github.com/dreschagin/monitoring-dashboard/internal/domain/entity"
)

// MetricAggregator предоставляет сервисы для агрегации метрик (Domain Service)
// Содержит бизнес-логику, которая не принадлежит одной конкретной сущности
type MetricAggregator struct{}

// NewMetricAggregator создает новый MetricAggregator
func NewMetricAggregator() *MetricAggregator {
	return &MetricAggregator{}
}

// CalculateAverage вычисляет среднее значение метрик
func (a *MetricAggregator) CalculateAverage(metrics []*entity.Metric) (float64, error) {
	if len(metrics) == 0 {
		return 0, errors.New("no metrics to aggregate")
	}

	var sum float64
	for _, m := range metrics {
		sum += m.Value().Raw()
	}

	return sum / float64(len(metrics)), nil
}

// CalculateMin находит минимальное значение среди метрик
func (a *MetricAggregator) CalculateMin(metrics []*entity.Metric) (float64, error) {
	if len(metrics) == 0 {
		return 0, errors.New("no metrics to aggregate")
	}

	min := metrics[0].Value().Raw()
	for _, m := range metrics[1:] {
		if val := m.Value().Raw(); val < min {
			min = val
		}
	}

	return min, nil
}

// CalculateMax находит максимальное значение среди метрик
func (a *MetricAggregator) CalculateMax(metrics []*entity.Metric) (float64, error) {
	if len(metrics) == 0 {
		return 0, errors.New("no metrics to aggregate")
	}

	max := metrics[0].Value().Raw()
	for _, m := range metrics[1:] {
		if val := m.Value().Raw(); val > max {
			max = val
		}
	}

	return max, nil
}

// FindPeaks находит метрики, превышающие указанный порог
func (a *MetricAggregator) FindPeaks(metrics []*entity.Metric, threshold float64) []*entity.Metric {
	var peaks []*entity.Metric
	for _, m := range metrics {
		if m.ExceedsThreshold(threshold) {
			peaks = append(peaks, m)
		}
	}
	return peaks
}

// FindCritical находит критические метрики
func (a *MetricAggregator) FindCritical(metrics []*entity.Metric) []*entity.Metric {
	var critical []*entity.Metric
	for _, m := range metrics {
		if m.IsCritical() {
			critical = append(critical, m)
		}
	}
	return critical
}

// FindWarning находит метрики с предупреждением
func (a *MetricAggregator) FindWarning(metrics []*entity.Metric) []*entity.Metric {
	var warning []*entity.Metric
	for _, m := range metrics {
		if m.IsWarning() && !m.IsCritical() {
			warning = append(warning, m)
		}
	}
	return warning
}

// SortByValue сортирует метрики по значению
func (a *MetricAggregator) SortByValue(metrics []*entity.Metric, descending bool) []*entity.Metric {
	sorted := make([]*entity.Metric, len(metrics))
	copy(sorted, metrics)

	sort.Slice(sorted, func(i, j int) bool {
		if descending {
			return sorted[i].Value().Raw() > sorted[j].Value().Raw()
		}
		return sorted[i].Value().Raw() < sorted[j].Value().Raw()
	})

	return sorted
}

// SortByTime сортирует метрики по времени сбора
func (a *MetricAggregator) SortByTime(metrics []*entity.Metric, descending bool) []*entity.Metric {
	sorted := make([]*entity.Metric, len(metrics))
	copy(sorted, metrics)

	sort.Slice(sorted, func(i, j int) bool {
		if descending {
			return sorted[i].CollectedAt().After(sorted[j].CollectedAt())
		}
		return sorted[i].CollectedAt().Before(sorted[j].CollectedAt())
	})

	return sorted
}

// CalculatePercentile вычисляет процентиль для метрик
func (a *MetricAggregator) CalculatePercentile(metrics []*entity.Metric, percentile float64) (float64, error) {
	if len(metrics) == 0 {
		return 0, errors.New("no metrics to aggregate")
	}

	if percentile < 0 || percentile > 100 {
		return 0, errors.New("percentile must be between 0 and 100")
	}

	sorted := a.SortByValue(metrics, false)
	index := int(float64(len(sorted)-1) * (percentile / 100.0))

	return sorted[index].Value().Raw(), nil
}
