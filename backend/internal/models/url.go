package models

import (
	"errors"
	"time"
)

// URL is the core domain model representing a shortened URL.
// Maps directly to the urls table in PostgreSQL.
type URL struct {
	ID         string     `json:"id"`
	ShortCode  string     `json:"short_code"`
	LongURL    string     `json:"long_url"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	ClickCount int64      `json:"click_count"`
}

// IsExpired reports whether this URL has passed its expiry time.
func (u *URL) IsExpired() bool {
	if u.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*u.ExpiresAt)
}

// CreateURLRequest is the payload for POST /api/v1/shorten.
type CreateURLRequest struct {
	LongURL     string  `json:"long_url"`
	CustomAlias *string `json:"custom_alias,omitempty"`
	// ExpiresInHours is optional. 0 means no expiry.
	ExpiresInHours int `json:"expires_in_hours,omitempty"`
}

// Validate performs basic input validation.
func (r *CreateURLRequest) Validate() error {
	if r.LongURL == "" {
		return errors.New("long_url is required")
	}
	if len(r.LongURL) > 2048 {
		return errors.New("long_url exceeds maximum length of 2048 characters")
	}
	if r.CustomAlias != nil {
		if len(*r.CustomAlias) < 3 || len(*r.CustomAlias) > 50 {
			return errors.New("custom_alias must be between 3 and 50 characters")
		}
	}
	if r.ExpiresInHours < 0 {
		return errors.New("expires_in_hours must be non-negative")
	}
	return nil
}

// CreateURLResponse is the API response for a newly created short URL.
type CreateURLResponse struct {
	ID        string     `json:"id"`
	ShortCode string     `json:"short_code"`
	ShortURL  string     `json:"short_url"`
	LongURL   string     `json:"long_url"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// AnalyticsResponse holds analytics data for a given short code.
type AnalyticsResponse struct {
	ShortCode      string     `json:"short_code"`
	LongURL        string     `json:"long_url"`
	ClickCount     int64      `json:"click_count"`
	CreatedAt      time.Time  `json:"created_at"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
}
