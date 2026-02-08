package port

import (
	"context"
)

// EventPublisher defines the interface for publishing events to a message broker
type EventPublisher interface {
	// PublishEvent publishes an event to the specified subject
	PublishEvent(ctx context.Context, subject string, event interface{}) error

	// Close closes the connection to the message broker
	Close() error
}
