package service

import (
	"errors"
	"time"

	"github.com/dreschagin/monitoring-dashboard/internal/domain/entity"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/valueobject"
)

// MetricValidator предоставляет сервисы для валидации метрик (Domain Service)
type MetricValidator struct{}

// NewMetricValidator создает новый MetricValidator
func NewMetricValidator() *MetricValidator {
	return &MetricValidator{}
}

// Validate выполняет полную валидацию метрики
func (v *MetricValidator) Validate(metric *entity.Metric) error {
	if metric == nil {
		return errors.New("metric cannot be nil")
	}

	// Проверка типа метрики
	if err := metric.Type().Validate(); err != nil {
		return err
	}

	// Проверка значения
	if metric.Value().Raw() < 0 {
		return errors.New("metric value cannot be negative")
	}

	// Проверка времени
	if metric.CollectedAt().IsZero() {
		return errors.New("collected_at cannot be zero")
	}

	// Проверка, что метрика не из будущего
	if metric.CollectedAt().After(time.Now()) {
		return errors.New("collected_at cannot be in the future")
	}

	// Проверка валидности единиц измерения для типа метрики
	if err := v.ValidateUnit(metric.Type(), metric.Value().Unit()); err != nil {
		return err
	}

	return nil
}

// ValidateUnit проверяет, соответствует ли единица измерения типу метрики
func (v *MetricValidator) ValidateUnit(metricType valueobject.MetricType, unit string) error {
	validUnits := map[valueobject.MetricType][]string{
		valueobject.CPU:     {"%"},
		valueobject.Memory:  {"%", "MB", "GB", "bytes"},
		valueobject.Disk:    {"%", "MB", "GB", "TB", "bytes"},
		valueobject.Network: {"KB/s", "MB/s", "GB/s", "bytes/s"},
	}

	allowedUnits, exists := validUnits[metricType]
	if !exists {
		return errors.New("unknown metric type")
	}

	for _, allowedUnit := range allowedUnits {
		if unit == allowedUnit {
			return nil
		}
	}

	return errors.New("invalid unit for metric type")
}

// ValidateBatch валидирует группу метрик
func (v *MetricValidator) ValidateBatch(metrics []*entity.Metric) []error {
	var errs []error

	for i, metric := range metrics {
		if err := v.Validate(metric); err != nil {
			errs = append(errs, errors.New("metric "+string(rune(i))+": "+err.Error()))
		}
	}

	return errs
}

// IsReasonable проверяет, находится ли значение метрики в разумных пределах
func (v *MetricValidator) IsReasonable(metric *entity.Metric) bool {
	switch metric.Type() {
	case valueobject.CPU, valueobject.Memory, valueobject.Disk:
		// Процентные значения должны быть от 0 до 100
		if metric.Value().Unit() == "%" {
			val := metric.Value().Raw()
			return val >= 0 && val <= 100
		}
		return true

	case valueobject.Network:
		// Сетевой трафик не должен быть чрезмерно большим (< 10 GB/s)
		if metric.Value().Unit() == "GB/s" {
			return metric.Value().Raw() < 10
		}
		if metric.Value().Unit() == "MB/s" {
			return metric.Value().Raw() < 10000
		}
		return true

	default:
		return true
	}
}
