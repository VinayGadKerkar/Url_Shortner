package repository

import (
	"context"
	"urlshortener/internal/models"
)

// URLRepository defines persistence operations for URLs.
// Concrete implementations: PostgresURLRepository, (future: DynamoDBURLRepository, etc.)
type URLRepository interface {
	// Create persists a new URL and returns the saved record.
	Create(ctx context.Context, url *models.URL) (*models.URL, error)

	// GetByShortCode looks up a URL by its short code.
	GetByShortCode(ctx context.Context, shortCode string) (*models.URL, error)

	// IncrementClickCount atomically increments the click counter and updates last_accessed_at.
	IncrementClickCount(ctx context.Context, shortCode string) error

	// ShortCodeExists checks whether a short code is already taken.
	ShortCodeExists(ctx context.Context, shortCode string) (bool, error)
}

// CacheRepository defines caching operations for URL lookups.
// Concrete implementation: RedisURLCache.
type CacheRepository interface {
	// Set stores a URL in the cache with the configured TTL.
	Set(ctx context.Context, shortCode string, url *models.URL) error

	// Get retrieves a URL from the cache. Returns nil, nil on cache miss.
	Get(ctx context.Context, shortCode string) (*models.URL, error)

	// Delete removes a URL from the cache.
	Delete(ctx context.Context, shortCode string) error

	// Ping checks connectivity to the cache backend.
	Ping(ctx context.Context) error
}
