package valueobject

import "errors"

// MetricType представляет тип метрики (Value Object)
type MetricType string

const (
	CPU     MetricType = "cpu"
	Memory  MetricType = "memory"
	Disk    MetricType = "disk"
	Network MetricType = "network"
)

// Validate проверяет валидность типа метрики
func (mt MetricType) Validate() error {
	switch mt {
	case CPU, Memory, Disk, Network:
		return nil
	default:
		return errors.New("invalid metric type")
	}
}

// String возвращает строковое представление типа метрики
func (mt MetricType) String() string {
	return string(mt)
}

// AllMetricTypes возвращает список всех допустимых типов метрик
func AllMetricTypes() []MetricType {
	return []MetricType{CPU, Memory, Disk, Network}
}
