package port

import "context"

// Cache defines the interface for caching operations
type Cache interface {
	// Get retrieves a value from cache
	Get(ctx context.Context, key string, dest interface{}) error

	// Set stores a value in cache
	Set(ctx context.Context, key string, value interface{}) error

	// Delete removes a value from cache
	Delete(ctx context.Context, key string) error

	// DeletePattern removes all keys matching pattern
	DeletePattern(ctx context.Context, pattern string) error

	// Close closes the cache connection
	Close() error
}
