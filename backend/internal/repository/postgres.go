package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"urlshortener/internal/models"
)

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("record not found")

// PostgresURLRepository implements URLRepository using PostgreSQL via pgxpool.
type PostgresURLRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresURLRepository creates a new repository with the provided connection pool.
func NewPostgresURLRepository(pool *pgxpool.Pool) *PostgresURLRepository {
	return &PostgresURLRepository{pool: pool}
}

// Create inserts a new URL record into the urls table.
func (r *PostgresURLRepository) Create(ctx context.Context, url *models.URL) (*models.URL, error) {
	query := `
		INSERT INTO urls (id, short_code, long_url, created_at, expires_at, click_count)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, short_code, long_url, created_at, expires_at, click_count
	`
	row := r.pool.QueryRow(ctx, query,
		url.ID,
		url.ShortCode,
		url.LongURL,
		url.CreatedAt,
		url.ExpiresAt,
		url.ClickCount,
	)

	var saved models.URL
	err := row.Scan(
		&saved.ID,
		&saved.ShortCode,
		&saved.LongURL,
		&saved.CreatedAt,
		&saved.ExpiresAt,
		&saved.ClickCount,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres create url: %w", err)
	}
	return &saved, nil
}

// GetByShortCode fetches a URL record by its short_code column.
func (r *PostgresURLRepository) GetByShortCode(ctx context.Context, shortCode string) (*models.URL, error) {
	query := `
		SELECT id, short_code, long_url, created_at, expires_at, click_count
		FROM urls
		WHERE short_code = $1
	`
	row := r.pool.QueryRow(ctx, query, shortCode)

	var url models.URL
	err := row.Scan(
		&url.ID,
		&url.ShortCode,
		&url.LongURL,
		&url.CreatedAt,
		&url.ExpiresAt,
		&url.ClickCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres get by short code: %w", err)
	}
	return &url, nil
}

// IncrementClickCount atomically bumps click_count and sets last_accessed_at.
func (r *PostgresURLRepository) IncrementClickCount(ctx context.Context, shortCode string) error {
	query := `
		UPDATE urls
		SET click_count    = click_count + 1,
		    last_accessed_at = $1
		WHERE short_code = $2
	`
	ct, err := r.pool.Exec(ctx, query, time.Now().UTC(), shortCode)
	if err != nil {
		return fmt.Errorf("postgres increment click count: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ShortCodeExists checks uniqueness before inserting a custom alias.
func (r *PostgresURLRepository) ShortCodeExists(ctx context.Context, shortCode string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM urls WHERE short_code = $1)`
	var exists bool
	err := r.pool.QueryRow(ctx, query, shortCode).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("postgres short code exists: %w", err)
	}
	return exists, nil
}
