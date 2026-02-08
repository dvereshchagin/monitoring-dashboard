package cloudwatch

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/dreschagin/monitoring-dashboard/internal/domain/entity"
)

const (
	// CloudWatch limits
	maxMetricsPerRequest = 1000
	maxRetries           = 3
	initialBackoff       = 100 * time.Millisecond
)

// MetricsPublisherConfig holds configuration for CloudWatch metrics publishing.
type MetricsPublisherConfig struct {
	Namespace         string            // CloudWatch namespace (e.g., "MonitoringDashboard/System")
	Region            string            // AWS region (e.g., "us-east-1")
	Endpoint          string            // Optional endpoint override (for LocalStack)
	AccessKeyID       string            // AWS access key
	SecretAccessKey   string            // AWS secret key
	DefaultDimensions map[string]string // Default dimensions added to all metrics
	BufferSize        int               // Buffer size before auto-flush
	FlushInterval     time.Duration     // Automatic flush interval
	StorageResolution int32             // Storage resolution in seconds (1 or 60)
}

// MetricsPublisher publishes metrics to AWS CloudWatch.
type MetricsPublisher struct {
	client            *cloudwatch.Client
	namespace         string
	defaultDimensions map[string]string
	storageResolution int32

	buffer     []*entity.Metric
	bufferSize int
	mu         sync.Mutex

	flushTicker *time.Ticker
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// NewMetricsPublisher creates a new CloudWatch metrics publisher.
func NewMetricsPublisher(ctx context.Context, cfg MetricsPublisherConfig) (*MetricsPublisher, error) {
	if cfg.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	if cfg.Region == "" {
		return nil, fmt.Errorf("region is required")
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 100
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 10 * time.Second
	}
	if cfg.StorageResolution != 1 && cfg.StorageResolution != 60 {
		cfg.StorageResolution = 60 // Default to standard resolution
	}

	// Build AWS config
	awsCfg, err := buildAWSConfig(ctx, cfg.Region, cfg.Endpoint, cfg.AccessKeyID, cfg.SecretAccessKey)
	if err != nil {
		return nil, fmt.Errorf("failed to build AWS config: %w", err)
	}

	// Create CloudWatch client
	client := cloudwatch.NewFromConfig(awsCfg)

	p := &MetricsPublisher{
		client:            client,
		namespace:         cfg.Namespace,
		defaultDimensions: cfg.DefaultDimensions,
		storageResolution: cfg.StorageResolution,
		buffer:            make([]*entity.Metric, 0, cfg.BufferSize),
		bufferSize:        cfg.BufferSize,
		flushTicker:       time.NewTicker(cfg.FlushInterval),
		stopCh:            make(chan struct{}),
	}

	// Start background flush goroutine
	p.wg.Add(1)
	go p.flushLoop()

	return p, nil
}

// PublishBatch publishes multiple metrics, buffering them for efficient batch operations.
func (p *MetricsPublisher) PublishBatch(ctx context.Context, metrics []*entity.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, metric := range metrics {
		p.buffer = append(p.buffer, metric)

		// Auto-flush if buffer is full
		if len(p.buffer) >= p.bufferSize {
			if err := p.flushBufferUnsafe(ctx); err != nil {
				return fmt.Errorf("failed to flush buffer: %w", err)
			}
		}
	}

	return nil
}

// PublishSingle publishes a single metric immediately without buffering.
func (p *MetricsPublisher) PublishSingle(ctx context.Context, metric *entity.Metric) error {
	if metric == nil {
		return fmt.Errorf("metric cannot be nil")
	}

	datum := p.convertToDatum(metric)
	return p.publishBatchWithRetry(ctx, []types.MetricDatum{datum})
}

// Flush forces immediate publication of all buffered metrics.
func (p *MetricsPublisher) Flush(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.flushBufferUnsafe(ctx)
}

// Close stops the background flush goroutine and flushes remaining metrics.
func (p *MetricsPublisher) Close(ctx context.Context) error {
	close(p.stopCh)
	p.flushTicker.Stop()
	p.wg.Wait()

	return p.Flush(ctx)
}

// flushLoop runs in a background goroutine and flushes the buffer periodically.
func (p *MetricsPublisher) flushLoop() {
	defer p.wg.Done()

	for {
		select {
		case <-p.flushTicker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := p.Flush(ctx); err != nil {
				// Log error but don't fail - we'll retry on next tick
				// In production, this should log to a proper logger
				_ = err
			}
			cancel()
		case <-p.stopCh:
			return
		}
	}
}

// flushBufferUnsafe flushes the buffer without locking (caller must hold lock).
func (p *MetricsPublisher) flushBufferUnsafe(ctx context.Context) error {
	if len(p.buffer) == 0 {
		return nil
	}

	// Convert all buffered metrics to CloudWatch MetricDatum
	data := make([]types.MetricDatum, 0, len(p.buffer))
	for _, metric := range p.buffer {
		data = append(data, p.convertToDatum(metric))
	}

	// Publish in chunks (CloudWatch limit: 1000 metrics/request)
	for i := 0; i < len(data); i += maxMetricsPerRequest {
		end := i + maxMetricsPerRequest
		if end > len(data) {
			end = len(data)
		}

		chunk := data[i:end]
		if err := p.publishBatchWithRetry(ctx, chunk); err != nil {
			return fmt.Errorf("failed to publish chunk: %w", err)
		}
	}

	// Clear buffer
	p.buffer = p.buffer[:0]

	return nil
}

// publishBatchWithRetry publishes a batch of metrics with exponential backoff retry.
func (p *MetricsPublisher) publishBatchWithRetry(ctx context.Context, data []types.MetricDatum) error {
	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt < maxRetries; attempt++ {
		input := &cloudwatch.PutMetricDataInput{
			Namespace:  aws.String(p.namespace),
			MetricData: data,
		}

		_, err := p.client.PutMetricData(ctx, input)
		if err == nil {
			return nil
		}

		lastErr = err

		// Exponential backoff before retry
		if attempt < maxRetries-1 {
			select {
			case <-time.After(backoff):
				backoff *= 2
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// convertToDatum converts a domain Metric entity to CloudWatch MetricDatum.
func (p *MetricsPublisher) convertToDatum(metric *entity.Metric) types.MetricDatum {
	timestamp := metric.CollectedAt()

	// Build dimensions
	dimensions := make([]types.Dimension, 0)

	// Add default dimensions
	for key, value := range p.defaultDimensions {
		dimensions = append(dimensions, types.Dimension{
			Name:  aws.String(key),
			Value: aws.String(value),
		})
	}

	// Add metric-specific dimensions
	dimensions = append(dimensions,
		types.Dimension{
			Name:  aws.String("MetricType"),
			Value: aws.String(string(metric.Type())),
		},
		types.Dimension{
			Name:  aws.String("MetricName"),
			Value: aws.String(metric.Name()),
		},
	)

	// Map unit to CloudWatch StandardUnit
	unit := mapUnit(metric.Value().Unit())

	datum := types.MetricDatum{
		MetricName: aws.String(metric.Name()),
		Value:      aws.Float64(metric.Value().Raw()),
		Unit:       unit,
		Timestamp:  aws.Time(timestamp),
		Dimensions: dimensions,
	}

	// Set storage resolution (high-resolution metrics)
	if p.storageResolution > 0 {
		datum.StorageResolution = aws.Int32(p.storageResolution)
	}

	return datum
}

// mapUnit maps metric units to CloudWatch StandardUnit.
func mapUnit(unit string) types.StandardUnit {
	switch unit {
	case "%":
		return types.StandardUnitPercent
	case "MB/s":
		return types.StandardUnitMegabytesSecond
	case "GB/s":
		return types.StandardUnitGigabytesSecond
	case "KB/s":
		return types.StandardUnitKilobytesSecond
	case "bytes":
		return types.StandardUnitBytes
	case "KB":
		return types.StandardUnitKilobytes
	case "MB":
		return types.StandardUnitMegabytes
	case "GB":
		return types.StandardUnitGigabytes
	case "ms":
		return types.StandardUnitMilliseconds
	case "s":
		return types.StandardUnitSeconds
	case "count":
		return types.StandardUnitCount
	default:
		return types.StandardUnitNone
	}
}

// buildAWSConfig creates an AWS config with credentials.
func buildAWSConfig(ctx context.Context, region, endpoint, accessKeyID, secretAccessKey string) (aws.Config, error) {
	optFns := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}

	// Add static credentials if provided
	if accessKeyID != "" && secretAccessKey != "" {
		optFns = append(optFns, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
		))
	}

	cfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return aws.Config{}, err
	}

	// Override endpoint if specified (for LocalStack testing)
	if endpoint != "" {
		cfg.BaseEndpoint = aws.String(endpoint)
	}

	return cfg, nil
}
