package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/dreschagin/monitoring-dashboard/internal/domain/entity"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/valueobject"
	_ "github.com/lib/pq"
)

// PostgresMetricRepository реализует repository.MetricRepository для PostgreSQL
type PostgresMetricRepository struct {
	db *sql.DB
}

// NewPostgresMetricRepository создает новый PostgreSQL repository
func NewPostgresMetricRepository(db *sql.DB) *PostgresMetricRepository {
	return &PostgresMetricRepository{
		db: db,
	}
}

// Save сохраняет одну метрику
func (r *PostgresMetricRepository) Save(ctx context.Context, metric *entity.Metric) error {
	model, err := ToDBModel(metric)
	if err != nil {
		return fmt.Errorf("failed to convert to DB model: %w", err)
	}

	query := `
		INSERT INTO metrics (id, metric_type, metric_name, value, unit, metadata, collected_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err = r.db.ExecContext(ctx, query,
		model.ID,
		model.MetricType,
		model.MetricName,
		model.Value,
		model.Unit,
		model.Metadata,
		model.CollectedAt,
		model.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert metric: %w", err)
	}

	return nil
}

// SaveBatch сохраняет несколько метрик одной транзакцией
func (r *PostgresMetricRepository) SaveBatch(ctx context.Context, metrics []*entity.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO metrics (id, metric_type, metric_name, value, unit, metadata, collected_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, metric := range metrics {
		model, err := ToDBModel(metric)
		if err != nil {
			return fmt.Errorf("failed to convert metric to DB model: %w", err)
		}

		_, err = stmt.ExecContext(ctx,
			model.ID,
			model.MetricType,
			model.MetricName,
			model.Value,
			model.Unit,
			model.Metadata,
			model.CollectedAt,
			model.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert metric: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// FindByID находит метрику по идентификатору
func (r *PostgresMetricRepository) FindByID(ctx context.Context, id string) (*entity.Metric, error) {
	query := `
		SELECT id, metric_type, metric_name, value, unit, metadata, collected_at, created_at
		FROM metrics
		WHERE id = $1
	`

	row := r.db.QueryRowContext(ctx, query, id)
	model, err := ScanMetricRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("metric not found: %s", id)
		}
		return nil, fmt.Errorf("failed to scan metric: %w", err)
	}

	return ToEntity(model)
}

// FindByType находит метрики по типу с ограничением количества
func (r *PostgresMetricRepository) FindByType(
	ctx context.Context,
	metricType valueobject.MetricType,
	limit int,
) ([]*entity.Metric, error) {
	query := `
		SELECT id, metric_type, metric_name, value, unit, metadata, collected_at, created_at
		FROM metrics
		WHERE metric_type = $1
		ORDER BY collected_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, metricType.String(), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}
	defer rows.Close()

	return r.scanMetrics(rows)
}

// FindByTimeRange находит метрики по типу и временному диапазону
func (r *PostgresMetricRepository) FindByTimeRange(
	ctx context.Context,
	metricType valueobject.MetricType,
	timeRange valueobject.TimeRange,
) ([]*entity.Metric, error) {
	query := `
		SELECT id, metric_type, metric_name, value, unit, metadata, collected_at, created_at
		FROM metrics
		WHERE metric_type = $1 AND collected_at BETWEEN $2 AND $3
		ORDER BY collected_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query,
		metricType.String(),
		timeRange.Start(),
		timeRange.End(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}
	defer rows.Close()

	return r.scanMetrics(rows)
}

// FindLatest находит последние метрики каждого типа
func (r *PostgresMetricRepository) FindLatest(ctx context.Context) (map[valueobject.MetricType]*entity.Metric, error) {
	query := `
		SELECT DISTINCT ON (metric_type)
			id, metric_type, metric_name, value, unit, metadata, collected_at, created_at
		FROM metrics
		ORDER BY metric_type, collected_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query latest metrics: %w", err)
	}
	defer rows.Close()

	metrics, err := r.scanMetrics(rows)
	if err != nil {
		return nil, err
	}

	// Преобразуем в map
	result := make(map[valueobject.MetricType]*entity.Metric)
	for _, metric := range metrics {
		result[metric.Type()] = metric
	}

	return result, nil
}

// FindLatestByType находит последнюю метрику указанного типа
func (r *PostgresMetricRepository) FindLatestByType(
	ctx context.Context,
	metricType valueobject.MetricType,
) (*entity.Metric, error) {
	query := `
		SELECT id, metric_type, metric_name, value, unit, metadata, collected_at, created_at
		FROM metrics
		WHERE metric_type = $1
		ORDER BY collected_at DESC
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, metricType.String())
	model, err := ScanMetricRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no metrics found for type: %s", metricType.String())
		}
		return nil, fmt.Errorf("failed to scan metric: %w", err)
	}

	return ToEntity(model)
}

// DeleteOlderThan удаляет метрики старше указанного времени
func (r *PostgresMetricRepository) DeleteOlderThan(ctx context.Context, timeRange valueobject.TimeRange) error {
	query := `
		DELETE FROM metrics
		WHERE collected_at < $1
	`

	result, err := r.db.ExecContext(ctx, query, timeRange.Start())
	if err != nil {
		return fmt.Errorf("failed to delete old metrics: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		// Log количество удаленных записей (можно добавить logger)
		_ = rowsAffected
	}

	return nil
}

// Count возвращает количество метрик по типу
func (r *PostgresMetricRepository) Count(ctx context.Context, metricType valueobject.MetricType) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM metrics
		WHERE metric_type = $1
	`

	var count int64
	err := r.db.QueryRowContext(ctx, query, metricType.String()).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count metrics: %w", err)
	}

	return count, nil
}

// scanMetrics сканирует несколько строк в слайс метрик
func (r *PostgresMetricRepository) scanMetrics(rows *sql.Rows) ([]*entity.Metric, error) {
	var metrics []*entity.Metric

	for rows.Next() {
		model, err := ScanMetricRow(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan metric row: %w", err)
		}

		metric, err := ToEntity(model)
		if err != nil {
			return nil, fmt.Errorf("failed to convert to entity: %w", err)
		}

		metrics = append(metrics, metric)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return metrics, nil
}
