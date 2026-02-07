package postgres

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/dreschagin/monitoring-dashboard/internal/domain/entity"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/valueobject"
)

// MetricDBModel представляет метрику в БД
type MetricDBModel struct {
	ID          string
	MetricType  string
	MetricName  string
	Value       float64
	Unit        string
	Metadata    []byte // JSON
	CollectedAt time.Time
	CreatedAt   time.Time
}

// ToDBModel конвертирует Domain Entity в DB Model
func ToDBModel(metric *entity.Metric) (*MetricDBModel, error) {
	var metadataBytes []byte
	var err error

	metadata := metric.Metadata()
	if len(metadata) > 0 {
		metadataBytes, err = json.Marshal(metadata)
		if err != nil {
			return nil, err
		}
	}

	return &MetricDBModel{
		ID:          metric.ID(),
		MetricType:  metric.Type().String(),
		MetricName:  metric.Name(),
		Value:       metric.Value().Raw(),
		Unit:        metric.Value().Unit(),
		Metadata:    metadataBytes,
		CollectedAt: metric.CollectedAt(),
		CreatedAt:   metric.CreatedAt(),
	}, nil
}

// ToEntity конвертирует DB Model в Domain Entity
func ToEntity(model *MetricDBModel) (*entity.Metric, error) {
	// Парсим metadata
	var metadata map[string]interface{}
	if len(model.Metadata) > 0 {
		if err := json.Unmarshal(model.Metadata, &metadata); err != nil {
			return nil, err
		}
	}

	// Создаем MetricType
	metricType := valueobject.MetricType(model.MetricType)

	// Создаем MetricValue
	metricValue, err := valueobject.NewMetricValue(model.Value, model.Unit)
	if err != nil {
		return nil, err
	}

	// Восстанавливаем entity через Reconstruct
	metric := entity.Reconstruct(
		model.ID,
		metricType,
		model.MetricName,
		metricValue,
		metadata,
		model.CollectedAt,
		model.CreatedAt,
	)

	return metric, nil
}

// ScanMetricRow сканирует строку БД в MetricDBModel
func ScanMetricRow(row interface {
	Scan(dest ...interface{}) error
}) (*MetricDBModel, error) {
	var model MetricDBModel
	var metadata sql.NullString

	err := row.Scan(
		&model.ID,
		&model.MetricType,
		&model.MetricName,
		&model.Value,
		&model.Unit,
		&metadata,
		&model.CollectedAt,
		&model.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	if metadata.Valid {
		model.Metadata = []byte(metadata.String)
	}

	return &model, nil
}
