-- +goose Up
-- +goose StatementBegin
-- Covering index for history queries - includes value and unit to avoid table lookups
CREATE INDEX IF NOT EXISTS idx_metrics_type_time_covering
    ON metrics(metric_type, collected_at DESC)
    INCLUDE (metric_name, value, unit, metadata);

-- Optimize DISTINCT ON queries for latest metrics
CREATE INDEX IF NOT EXISTS idx_metrics_type_time_id
    ON metrics(metric_type, collected_at DESC, id);

-- Add partial index for recent data (last 7 days) - speeds up most common queries
CREATE INDEX IF NOT EXISTS idx_metrics_recent
    ON metrics(metric_type, collected_at DESC)
    WHERE collected_at > NOW() - INTERVAL '7 days';

-- Create materialized view for aggregated metrics (hourly averages)
CREATE MATERIALIZED VIEW IF NOT EXISTS metrics_hourly AS
SELECT
    metric_type,
    metric_name,
    DATE_TRUNC('hour', collected_at) as hour_bucket,
    AVG(value) as avg_value,
    MIN(value) as min_value,
    MAX(value) as max_value,
    COUNT(*) as sample_count,
    unit
FROM metrics
WHERE collected_at > NOW() - INTERVAL '30 days'
GROUP BY metric_type, metric_name, hour_bucket, unit;

CREATE UNIQUE INDEX IF NOT EXISTS idx_metrics_hourly_unique
    ON metrics_hourly(metric_type, metric_name, hour_bucket);

CREATE INDEX IF NOT EXISTS idx_metrics_hourly_time
    ON metrics_hourly(hour_bucket DESC);

COMMENT ON MATERIALIZED VIEW metrics_hourly IS 'Hourly aggregated metrics for faster historical queries';
COMMENT ON INDEX idx_metrics_type_time_covering IS 'Covering index to avoid table lookups for history queries';
COMMENT ON INDEX idx_metrics_recent IS 'Partial index for recent data (last 7 days) - most queried';

-- Function to refresh materialized view (call this hourly via cron/scheduler)
CREATE OR REPLACE FUNCTION refresh_metrics_hourly()
RETURNS void AS $$
BEGIN
    REFRESH MATERIALIZED VIEW CONCURRENTLY metrics_hourly;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION refresh_metrics_hourly IS 'Refresh hourly metrics materialized view - run every hour';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS refresh_metrics_hourly();
DROP MATERIALIZED VIEW IF EXISTS metrics_hourly;
DROP INDEX IF EXISTS idx_metrics_recent;
DROP INDEX IF EXISTS idx_metrics_type_time_id;
DROP INDEX IF EXISTS idx_metrics_type_time_covering;
-- +goose StatementEnd
