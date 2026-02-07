package valueobject

import (
	"errors"
	"fmt"
)

// MetricValue представляет значение метрики с единицей измерения (Value Object)
// Иммутабельный объект
type MetricValue struct {
	value float64
	unit  string
}

// NewMetricValue создает новый MetricValue с валидацией
func NewMetricValue(value float64, unit string) (MetricValue, error) {
	if value < 0 {
		return MetricValue{}, errors.New("value cannot be negative")
	}

	if unit == "" {
		return MetricValue{}, errors.New("unit cannot be empty")
	}

	return MetricValue{
		value: value,
		unit:  unit,
	}, nil
}

// Raw возвращает числовое значение
func (mv MetricValue) Raw() float64 {
	return mv.value
}

// Unit возвращает единицу измерения
func (mv MetricValue) Unit() string {
	return mv.unit
}

// String возвращает строковое представление
func (mv MetricValue) String() string {
	return fmt.Sprintf("%.2f %s", mv.value, mv.unit)
}

// Equals сравнивает два MetricValue
func (mv MetricValue) Equals(other MetricValue) bool {
	return mv.value == other.value && mv.unit == other.unit
}
