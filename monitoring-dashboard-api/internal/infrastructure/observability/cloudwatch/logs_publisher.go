package cloudwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	applicationPort "github.com/dreschagin/monitoring-dashboard/internal/application/port"
)

const (
	// CloudWatch Logs limits
	maxLogEventsPerRequest = 10000
	maxLogBatchSize        = 1048576 // 1 MB
	maxLogEventSize        = 256000  // 256 KB
)

// LogsPublisherConfig holds configuration for CloudWatch logs publishing.
type LogsPublisherConfig struct {
	LogGroupName    string // CloudWatch log group name
	LogStreamName   string // CloudWatch log stream name
	Region          string // AWS region
	Endpoint        string // Optional endpoint override (for LocalStack)
	AccessKeyID     string // AWS access key
	SecretAccessKey string // AWS secret key
	BufferSize      int    // Buffer size before auto-flush
	FlushInterval   time.Duration
	AutoCreate      bool // Automatically create log group/stream if missing
}

// LogsPublisher publishes logs to AWS CloudWatch Logs.
type LogsPublisher struct {
	client        *cloudwatchlogs.Client
	logGroupName  string
	logStreamName string
	autoCreate    bool

	buffer     []applicationPort.LogEntry
	bufferSize int
	mu         sync.Mutex

	sequenceToken *string // CloudWatch requires sequence tokens for ordering

	flushTicker *time.Ticker
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// NewLogsPublisher creates a new CloudWatch logs publisher.
func NewLogsPublisher(ctx context.Context, cfg LogsPublisherConfig) (*LogsPublisher, error) {
	if cfg.LogGroupName == "" {
		return nil, fmt.Errorf("log group name is required")
	}
	if cfg.LogStreamName == "" {
		return nil, fmt.Errorf("log stream name is required")
	}
	if cfg.Region == "" {
		return nil, fmt.Errorf("region is required")
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 50
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}

	// Build AWS config
	awsCfg, err := buildAWSConfig(ctx, cfg.Region, cfg.Endpoint, cfg.AccessKeyID, cfg.SecretAccessKey)
	if err != nil {
		return nil, fmt.Errorf("failed to build AWS config: %w", err)
	}

	// Create CloudWatch Logs client
	client := cloudwatchlogs.NewFromConfig(awsCfg)

	p := &LogsPublisher{
		client:        client,
		logGroupName:  cfg.LogGroupName,
		logStreamName: cfg.LogStreamName,
		autoCreate:    cfg.AutoCreate,
		buffer:        make([]applicationPort.LogEntry, 0, cfg.BufferSize),
		bufferSize:    cfg.BufferSize,
		flushTicker:   time.NewTicker(cfg.FlushInterval),
		stopCh:        make(chan struct{}),
	}

	// Ensure log group and stream exist if auto-create is enabled
	if cfg.AutoCreate {
		if err := p.ensureLogGroupAndStream(ctx); err != nil {
			return nil, fmt.Errorf("failed to create log group/stream: %w", err)
		}
	}

	// Start background flush goroutine
	p.wg.Add(1)
	go p.flushLoop()

	return p, nil
}

// Publish publishes a single log entry, buffering it for efficient batch operations.
func (p *LogsPublisher) Publish(ctx context.Context, entry applicationPort.LogEntry) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.buffer = append(p.buffer, entry)

	// Auto-flush if buffer is full
	if len(p.buffer) >= p.bufferSize {
		if err := p.flushBufferUnsafe(ctx); err != nil {
			return fmt.Errorf("failed to flush buffer: %w", err)
		}
	}

	return nil
}

// PublishBatch publishes multiple log entries, buffering them.
func (p *LogsPublisher) PublishBatch(ctx context.Context, entries []applicationPort.LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, entry := range entries {
		p.buffer = append(p.buffer, entry)

		// Auto-flush if buffer is full
		if len(p.buffer) >= p.bufferSize {
			if err := p.flushBufferUnsafe(ctx); err != nil {
				return fmt.Errorf("failed to flush buffer: %w", err)
			}
		}
	}

	return nil
}

// Flush forces immediate publication of all buffered log entries.
func (p *LogsPublisher) Flush(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.flushBufferUnsafe(ctx)
}

// Close stops the background flush goroutine and flushes remaining logs.
func (p *LogsPublisher) Close(ctx context.Context) error {
	close(p.stopCh)
	p.flushTicker.Stop()
	p.wg.Wait()

	return p.Flush(ctx)
}

// flushLoop runs in a background goroutine and flushes the buffer periodically.
func (p *LogsPublisher) flushLoop() {
	defer p.wg.Done()

	for {
		select {
		case <-p.flushTicker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := p.Flush(ctx); err != nil {
				// Log error but don't fail - we'll retry on next tick
				_ = err
			}
			cancel()
		case <-p.stopCh:
			return
		}
	}
}

// flushBufferUnsafe flushes the buffer without locking (caller must hold lock).
func (p *LogsPublisher) flushBufferUnsafe(ctx context.Context) error {
	if len(p.buffer) == 0 {
		return nil
	}

	// Sort by timestamp (CloudWatch Logs requirement)
	sort.Slice(p.buffer, func(i, j int) bool {
		return p.buffer[i].Timestamp.Before(p.buffer[j].Timestamp)
	})

	// Convert to CloudWatch log events
	events := make([]types.InputLogEvent, 0, len(p.buffer))
	for _, entry := range p.buffer {
		event, err := p.convertToLogEvent(entry)
		if err != nil {
			// Skip malformed entries but don't fail the entire batch
			continue
		}
		events = append(events, event)
	}

	if len(events) == 0 {
		p.buffer = p.buffer[:0]
		return nil
	}

	// Publish in chunks (CloudWatch Logs limit: 10,000 events/request)
	for i := 0; i < len(events); i += maxLogEventsPerRequest {
		end := i + maxLogEventsPerRequest
		if end > len(events) {
			end = len(events)
		}

		chunk := events[i:end]
		if err := p.publishLogEventsWithRetry(ctx, chunk); err != nil {
			return fmt.Errorf("failed to publish chunk: %w", err)
		}
	}

	// Clear buffer
	p.buffer = p.buffer[:0]

	return nil
}

// publishLogEventsWithRetry publishes log events with retry logic.
func (p *LogsPublisher) publishLogEventsWithRetry(ctx context.Context, events []types.InputLogEvent) error {
	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt < maxRetries; attempt++ {
		input := &cloudwatchlogs.PutLogEventsInput{
			LogGroupName:  aws.String(p.logGroupName),
			LogStreamName: aws.String(p.logStreamName),
			LogEvents:     events,
			SequenceToken: p.sequenceToken,
		}

		output, err := p.client.PutLogEvents(ctx, input)
		if err == nil {
			// Update sequence token for next request
			p.sequenceToken = output.NextSequenceToken
			return nil
		}

		// Handle InvalidSequenceTokenException by retrying with the expected token
		var invalidSeqErr *types.InvalidSequenceTokenException
		if ok := attemptErrorAs(err, &invalidSeqErr); ok {
			p.sequenceToken = invalidSeqErr.ExpectedSequenceToken
			// Retry immediately with correct token
			continue
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

// convertToLogEvent converts a LogEntry to CloudWatch InputLogEvent.
func (p *LogsPublisher) convertToLogEvent(entry applicationPort.LogEntry) (types.InputLogEvent, error) {
	// Build structured JSON log
	logData := map[string]interface{}{
		"timestamp": entry.Timestamp.Format(time.RFC3339Nano),
		"level":     string(entry.Level),
		"message":   entry.Message,
	}

	// Add fields if present
	if len(entry.Fields) > 0 {
		logData["fields"] = entry.Fields
	}

	messageJSON, err := json.Marshal(logData)
	if err != nil {
		return types.InputLogEvent{}, fmt.Errorf("failed to marshal log entry: %w", err)
	}

	// Truncate if exceeds CloudWatch limit
	message := string(messageJSON)
	if len(message) > maxLogEventSize {
		message = message[:maxLogEventSize-3] + "..."
	}

	return types.InputLogEvent{
		Message:   aws.String(message),
		Timestamp: aws.Int64(entry.Timestamp.UnixMilli()),
	}, nil
}

// ensureLogGroupAndStream creates the log group and stream if they don't exist.
func (p *LogsPublisher) ensureLogGroupAndStream(ctx context.Context) error {
	// Create log group
	_, err := p.client.CreateLogGroup(ctx, &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String(p.logGroupName),
	})
	if err != nil {
		// Ignore error if log group already exists
		var alreadyExists *types.ResourceAlreadyExistsException
		if !attemptErrorAs(err, &alreadyExists) {
			return fmt.Errorf("failed to create log group: %w", err)
		}
	}

	// Create log stream
	_, err = p.client.CreateLogStream(ctx, &cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String(p.logGroupName),
		LogStreamName: aws.String(p.logStreamName),
	})
	if err != nil {
		// Ignore error if log stream already exists
		var alreadyExists *types.ResourceAlreadyExistsException
		if !attemptErrorAs(err, &alreadyExists) {
			return fmt.Errorf("failed to create log stream: %w", err)
		}
	}

	return nil
}

// attemptErrorAs is a helper for error type assertion that works with AWS SDK v2 errors.
func attemptErrorAs(err error, target interface{}) bool {
	if err == nil {
		return false
	}
	// Simple type assertion - AWS SDK v2 errors can be checked directly
	switch v := target.(type) {
	case **types.InvalidSequenceTokenException:
		if e, ok := err.(*types.InvalidSequenceTokenException); ok {
			*v = e
			return true
		}
	case **types.ResourceAlreadyExistsException:
		if e, ok := err.(*types.ResourceAlreadyExistsException); ok {
			*v = e
			return true
		}
	}
	return false
}
