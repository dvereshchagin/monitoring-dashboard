-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    metric_type VARCHAR(20) NOT NULL CHECK (metric_type IN ('cpu', 'memory', 'disk', 'network')),
    metric_name VARCHAR(50) NOT NULL,
    value NUMERIC(15,2) NOT NULL CHECK (value >= 0),
    unit VARCHAR(10) NOT NULL,
    metadata JSONB,
    collected_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE metrics IS 'System metrics collected over time';
COMMENT ON COLUMN metrics.metric_type IS 'Type of metric: cpu, memory, disk, network';
COMMENT ON COLUMN metrics.metric_name IS 'Specific name of the metric (e.g., cpu_usage, mem_used)';
COMMENT ON COLUMN metrics.value IS 'Numeric value of the metric';
COMMENT ON COLUMN metrics.unit IS 'Unit of measurement (%, MB, KB/s, etc.)';
COMMENT ON COLUMN metrics.metadata IS 'Additional metadata in JSON format';
COMMENT ON COLUMN metrics.collected_at IS 'When the metric was collected from the system';
COMMENT ON COLUMN metrics.created_at IS 'When the record was created in the database';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS metrics;
-- +goose StatementEnd
