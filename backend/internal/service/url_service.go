package service

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"urlshortener/internal/models"
	"urlshortener/internal/repository"
)

// ErrShortCodeConflict is returned when a custom alias is already taken.
var ErrShortCodeConflict = errors.New("short code already in use")

// ErrURLExpired is returned when the requested short code has expired.
var ErrURLExpired = errors.New("url has expired")

// URLService contains the business logic for creating and resolving short URLs.
// It orchestrates the repository (PostgreSQL) and cache (Redis) layers.
type URLService struct {
	repo   repository.URLRepository
	cache  repository.CacheRepository
	logger *zap.Logger
	// baseURL is prepended when building the full short URL in responses.
	baseURL string
}

// NewURLService creates a new URLService.
func NewURLService(
	repo repository.URLRepository,
	cache repository.CacheRepository,
	logger *zap.Logger,
	baseURL string,
) *URLService {
	return &URLService{
		repo:    repo,
		cache:   cache,
		logger:  logger,
		baseURL: baseURL,
	}
}

// CreateShortURL handles the create flow:
//  1. Validate input (delegated to model)
//  2. Generate or validate short code
//  3. Persist to PostgreSQL
//  4. Warm the Redis cache
func (s *URLService) CreateShortURL(ctx context.Context, req *models.CreateURLRequest) (*models.CreateURLResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}

	var shortCode string

	if req.CustomAlias != nil {
		// Custom alias path: check uniqueness first.
		exists, err := s.repo.ShortCodeExists(ctx, *req.CustomAlias)
		if err != nil {
			return nil, fmt.Errorf("checking short code: %w", err)
		}
		if exists {
			return nil, ErrShortCodeConflict
		}
		shortCode = *req.CustomAlias
	} else {
		// Auto-generate: preserve the existing MD5-based approach from the original code.
		shortCode = generateShortCode(req.LongURL)

		// Handle the (unlikely) collision by appending a nonce and retrying once.
		exists, err := s.repo.ShortCodeExists(ctx, shortCode)
		if err != nil {
			return nil, fmt.Errorf("checking short code: %w", err)
		}
		if exists {
			shortCode = generateShortCode(req.LongURL + uuid.New().String())
		}
	}

	url := &models.URL{
		ID:         uuid.New().String(),
		ShortCode:  shortCode,
		LongURL:    req.LongURL,
		CreatedAt:  time.Now().UTC(),
		ClickCount: 0,
	}

	if req.ExpiresInHours > 0 {
		t := time.Now().UTC().Add(time.Duration(req.ExpiresInHours) * time.Hour)
		url.ExpiresAt = &t
	}

	saved, err := s.repo.Create(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("persisting url: %w", err)
	}

	// Warm cache — best-effort; a failure here doesn't fail the request.
	if cacheErr := s.cache.Set(ctx, saved.ShortCode, saved); cacheErr != nil {
		s.logger.Warn("failed to warm cache after create", zap.Error(cacheErr), zap.String("short_code", saved.ShortCode))
	}

	return &models.CreateURLResponse{
		ID:        saved.ID,
		ShortCode: saved.ShortCode,
		ShortURL:  fmt.Sprintf("%s/%s", s.baseURL, saved.ShortCode),
		LongURL:   saved.LongURL,
		CreatedAt: saved.CreatedAt,
		ExpiresAt: saved.ExpiresAt,
	}, nil
}

// Resolve implements the cache-aside redirect flow:
//  1. Check Redis cache → hit: return immediately
//  2. Cache miss → query PostgreSQL
//  3. Check expiry
//  4. Re-populate cache
//
// The caller (handler) is responsible for publishing the Kafka click event.
func (s *URLService) Resolve(ctx context.Context, shortCode string) (*models.URL, error) {
	// --- Cache check ---
	cached, err := s.cache.Get(ctx, shortCode)
	if err != nil {
		// Cache error is non-fatal; fall through to DB.
		s.logger.Warn("cache get error, falling back to db", zap.Error(err), zap.String("short_code", shortCode))
	}
	if cached != nil {
		s.logger.Debug("cache hit", zap.String("short_code", shortCode))
		if cached.IsExpired() {
			return nil, ErrURLExpired
		}
		return cached, nil
	}

	// --- DB lookup ---
	s.logger.Debug("cache miss, querying db", zap.String("short_code", shortCode))
	url, err := s.repo.GetByShortCode(ctx, shortCode)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, fmt.Errorf("db lookup: %w", err)
	}

	if url.IsExpired() {
		return nil, ErrURLExpired
	}

	// Re-populate cache — best-effort.
	if cacheErr := s.cache.Set(ctx, url.ShortCode, url); cacheErr != nil {
		s.logger.Warn("failed to repopulate cache", zap.Error(cacheErr), zap.String("short_code", shortCode))
	}

	return url, nil
}

// GetAnalytics returns analytics data for a short code.
func (s *URLService) GetAnalytics(ctx context.Context, shortCode string) (*models.AnalyticsResponse, error) {
	url, err := s.repo.GetByShortCode(ctx, shortCode)
	if err != nil {
		return nil, err
	}
	return &models.AnalyticsResponse{
		ShortCode:  url.ShortCode,
		LongURL:    url.LongURL,
		ClickCount: url.ClickCount,
		CreatedAt:  url.CreatedAt,
		ExpiresAt:  url.ExpiresAt,
	}, nil
}

// generateShortCode preserves the original MD5-based logic from main.go.
// Takes the first 7 characters of the hex-encoded MD5 hash of the input.
func generateShortCode(input string) string {
	hasher := md5.New()
	hasher.Write([]byte(input))
	return hex.EncodeToString(hasher.Sum(nil))[:7]
}
