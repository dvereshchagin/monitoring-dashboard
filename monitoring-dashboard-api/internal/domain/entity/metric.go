package entity

import (
	"time"

	"github.com/dreschagin/monitoring-dashboard/internal/domain/valueobject"
	"github.com/google/uuid"
)

// Metric представляет метрику системы (Aggregate Root)
// Содержит бизнес-логику для работы с метриками
type Metric struct {
	id          string
	metricType  valueobject.MetricType
	metricName  string
	value       valueobject.MetricValue
	metadata    map[string]interface{}
	collectedAt time.Time
	createdAt   time.Time
}

// NewMetric создает новую метрику (Factory Method)
func NewMetric(
	metricType valueobject.MetricType,
	metricName string,
	value valueobject.MetricValue,
) (*Metric, error) {
	// Валидация типа метрики
	if err := metricType.Validate(); err != nil {
		return nil, err
	}

	now := time.Now()

	return &Metric{
		id:          uuid.New().String(),
		metricType:  metricType,
		metricName:  metricName,
		value:       value,
		metadata:    make(map[string]interface{}),
		collectedAt: now,
		createdAt:   now,
	}, nil
}

// Reconstruct восстанавливает метрику из хранилища (для Repository)
func Reconstruct(
	id string,
	metricType valueobject.MetricType,
	metricName string,
	value valueobject.MetricValue,
	metadata map[string]interface{},
	collectedAt, createdAt time.Time,
) *Metric {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	return &Metric{
		id:          id,
		metricType:  metricType,
		metricName:  metricName,
		value:       value,
		metadata:    metadata,
		collectedAt: collectedAt,
		createdAt:   createdAt,
	}
}

// ID возвращает идентификатор метрики
func (m *Metric) ID() string {
	return m.id
}

// Type возвращает тип метрики
func (m *Metric) Type() valueobject.MetricType {
	return m.metricType
}

// Name возвращает имя метрики
func (m *Metric) Name() string {
	return m.metricName
}

// Value возвращает значение метрики
func (m *Metric) Value() valueobject.MetricValue {
	return m.value
}

// Metadata возвращает метаданные
func (m *Metric) Metadata() map[string]interface{} {
	// Возвращаем копию для иммутабельности
	result := make(map[string]interface{}, len(m.metadata))
	for k, v := range m.metadata {
		result[k] = v
	}
	return result
}

// CollectedAt возвращает время сбора метрики
func (m *Metric) CollectedAt() time.Time {
	return m.collectedAt
}

// CreatedAt возвращает время создания записи
func (m *Metric) CreatedAt() time.Time {
	return m.createdAt
}

// SetMetadata устанавливает метаданные
func (m *Metric) SetMetadata(key string, value interface{}) {
	m.metadata[key] = value
}

// Domain Methods (бизнес-логика)

// IsStale проверяет, устарела ли метрика
func (m *Metric) IsStale(threshold time.Duration) bool {
	return time.Since(m.collectedAt) > threshold
}

// ExceedsThreshold проверяет, превышает ли значение метрики порог
func (m *Metric) ExceedsThreshold(threshold float64) bool {
	return m.value.Raw() > threshold
}

// IsCritical проверяет, является ли значение метрики критическим
// (Для CPU, Memory, Disk - более 90%)
func (m *Metric) IsCritical() bool {
	switch m.metricType {
	case valueobject.CPU, valueobject.Memory, valueobject.Disk:
		return m.value.Raw() > 90.0 && m.value.Unit() == "%"
	case valueobject.Network:
		// Для сети критическим считается более 100 MB/s
		return m.value.Raw() > 100.0 && m.value.Unit() == "MB/s"
	default:
		return false
	}
}

// IsWarning проверяет, является ли значение метрики предупреждающим
// (Для CPU, Memory, Disk - более 75%)
func (m *Metric) IsWarning() bool {
	switch m.metricType {
	case valueobject.CPU, valueobject.Memory, valueobject.Disk:
		return m.value.Raw() > 75.0 && m.value.Unit() == "%"
	case valueobject.Network:
		// Для сети предупреждение при более 50 MB/s
		return m.value.Raw() > 50.0 && m.value.Unit() == "MB/s"
	default:
		return false
	}
}

// Age возвращает возраст метрики с момента сбора
func (m *Metric) Age() time.Duration {
	return time.Since(m.collectedAt)
}
