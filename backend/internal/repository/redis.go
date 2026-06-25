package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"urlshortener/internal/models"
)

const cacheKeyPrefix = "url:"

// RedisURLCache implements CacheRepository using Redis.
// It follows the cache-aside pattern:
//
//	Read  → check cache first; on miss, caller queries DB and calls Set.
//	Write → invalidate/update cache after DB write.
type RedisURLCache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisURLCache creates a new Redis-backed cache repository.
func NewRedisURLCache(client *redis.Client, ttl time.Duration) *RedisURLCache {
	return &RedisURLCache{client: client, ttl: ttl}
}

// Set serialises a URL and stores it in Redis under "url:<shortCode>".
func (c *RedisURLCache) Set(ctx context.Context, shortCode string, url *models.URL) error {
	data, err := json.Marshal(url)
	if err != nil {
		return fmt.Errorf("redis cache set marshal: %w", err)
	}

	// If the URL has an explicit expiry, honour it as the cache TTL so we never
	// serve a stale redirect beyond the configured expiry time.
	ttl := c.ttl
	if url.ExpiresAt != nil {
		remaining := time.Until(*url.ExpiresAt)
		if remaining <= 0 {
			// Already expired — don't cache it at all.
			return nil
		}
		if remaining < ttl {
			ttl = remaining
		}
	}

	if err := c.client.Set(ctx, cacheKey(shortCode), data, ttl).Err(); err != nil {
		return fmt.Errorf("redis cache set: %w", err)
	}
	return nil
}

// Get deserialises a URL from Redis. Returns nil, nil on cache miss.
func (c *RedisURLCache) Get(ctx context.Context, shortCode string) (*models.URL, error) {
	data, err := c.client.Get(ctx, cacheKey(shortCode)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// Cache miss — not an error, just an absent entry.
			return nil, nil
		}
		return nil, fmt.Errorf("redis cache get: %w", err)
	}

	var url models.URL
	if err := json.Unmarshal(data, &url); err != nil {
		return nil, fmt.Errorf("redis cache get unmarshal: %w", err)
	}
	return &url, nil
}

// Delete removes the cache entry for shortCode (e.g., after a URL is updated/deleted).
func (c *RedisURLCache) Delete(ctx context.Context, shortCode string) error {
	if err := c.client.Del(ctx, cacheKey(shortCode)).Err(); err != nil {
		return fmt.Errorf("redis cache delete: %w", err)
	}
	return nil
}

// Ping checks connectivity to the Redis server.
func (c *RedisURLCache) Ping(ctx context.Context) error {
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	return nil
}

func cacheKey(shortCode string) string {
	return cacheKeyPrefix + shortCode
}
