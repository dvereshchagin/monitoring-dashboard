package port

import (
	"context"
	"time"
)

// LogLevel represents the severity of a log entry.
type LogLevel string

const (
	LogLevelDebug LogLevel = "DEBUG"
	LogLevelInfo  LogLevel = "INFO"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelError LogLevel = "ERROR"
)

// LogEntry represents a structured log entry for publishing to external log systems.
type LogEntry struct {
	Timestamp time.Time              // When the log event occurred
	Level     LogLevel               // Severity level
	Message   string                 // Log message
	Fields    map[string]interface{} // Additional structured fields
}

// LogPublisher defines the interface for publishing logs to external observability platforms.
// This port allows the application layer to publish logs without coupling to specific implementations.
type LogPublisher interface {
	// Publish sends a single log entry to the external system.
	Publish(ctx context.Context, entry LogEntry) error

	// PublishBatch sends multiple log entries in a single operation.
	// Implementations should handle batching constraints (e.g., CloudWatch's 10,000 events/request limit).
	PublishBatch(ctx context.Context, entries []LogEntry) error

	// Flush forces immediate publication of any buffered log entries.
	// Should be called during graceful shutdown to prevent data loss.
	Flush(ctx context.Context) error
}
