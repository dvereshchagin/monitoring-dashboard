package port

import (
	"context"

	"github.com/dreschagin/monitoring-dashboard/internal/domain/entity"
)

// MetricsPublisher defines the interface for publishing metrics to external observability platforms.
// This port allows the application layer to publish metrics without coupling to specific implementations.
type MetricsPublisher interface {
	// PublishBatch publishes multiple metrics in a single operation.
	// Implementations should handle batching constraints (e.g., CloudWatch's 1000 metrics/request limit).
	PublishBatch(ctx context.Context, metrics []*entity.Metric) error

	// PublishSingle publishes a single metric immediately.
	// Use this for high-priority metrics that need immediate delivery.
	PublishSingle(ctx context.Context, metric *entity.Metric) error

	// Flush forces immediate publication of any buffered metrics.
	// Should be called during graceful shutdown to prevent data loss.
	Flush(ctx context.Context) error
}
