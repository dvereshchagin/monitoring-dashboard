-- +goose Up
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_metrics_type_collected_at
    ON metrics(metric_type, collected_at DESC);

CREATE INDEX IF NOT EXISTS idx_metrics_collected_at
    ON metrics(collected_at DESC);

CREATE INDEX IF NOT EXISTS idx_metrics_name_collected_at
    ON metrics(metric_name, collected_at DESC);

CREATE INDEX IF NOT EXISTS idx_metrics_metadata_gin
    ON metrics USING GIN (metadata);

CREATE INDEX IF NOT EXISTS idx_metrics_created_at
    ON metrics(created_at DESC);

COMMENT ON INDEX idx_metrics_type_collected_at IS 'Optimizes queries for latest metrics by type';
COMMENT ON INDEX idx_metrics_collected_at IS 'Optimizes time-range queries';
COMMENT ON INDEX idx_metrics_name_collected_at IS 'Optimizes queries by metric name';
COMMENT ON INDEX idx_metrics_metadata_gin IS 'Optimizes JSONB metadata queries';
COMMENT ON INDEX idx_metrics_created_at IS 'Optimizes cleanup operations';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_metrics_created_at;
DROP INDEX IF EXISTS idx_metrics_metadata_gin;
DROP INDEX IF EXISTS idx_metrics_name_collected_at;
DROP INDEX IF EXISTS idx_metrics_collected_at;
DROP INDEX IF EXISTS idx_metrics_type_collected_at;
-- +goose StatementEnd
