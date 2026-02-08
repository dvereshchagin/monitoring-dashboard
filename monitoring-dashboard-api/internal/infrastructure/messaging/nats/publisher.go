package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dreschagin/monitoring-dashboard/pkg/logger"
	"github.com/nats-io/nats.go"
)

// NATSPublisher implements EventPublisher for NATS JetStream
type NATSPublisher struct {
	nc     *nats.Conn
	js     nats.JetStreamContext
	logger *logger.Logger
}

// NewNATSPublisher creates a new NATS publisher
func NewNATSPublisher(natsURL string, log *logger.Logger) (*NATSPublisher, error) {
	// Connect to NATS with retry
	nc, err := nats.Connect(natsURL,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(10),
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				log.Warn("NATS disconnected", "error", err.Error())
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Info("NATS reconnected", "url", nc.ConnectedUrl())
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Get JetStream context
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to get JetStream context: %w", err)
	}

	log.Info("Connected to NATS", "url", natsURL)

	return &NATSPublisher{
		nc:     nc,
		js:     js,
		logger: log,
	}, nil
}

// PublishEvent publishes an event to NATS (async)
func (p *NATSPublisher) PublishEvent(ctx context.Context, subject string, event interface{}) error {
	// Marshal event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Async publish (fire-and-forget for better performance)
	_, err = p.js.PublishAsync(subject, data)
	if err != nil {
		p.logger.Error("Failed to publish event", err,
			"subject", subject,
		)
		return fmt.Errorf("failed to publish event: %w", err)
	}

	p.logger.Debug("Event published",
		"subject", subject,
		"size", len(data),
	)

	return nil
}

// Close closes the NATS connection
func (p *NATSPublisher) Close() error {
	if p.nc != nil {
		p.logger.Info("Closing NATS connection")
		p.nc.Close()
	}
	return nil
}
