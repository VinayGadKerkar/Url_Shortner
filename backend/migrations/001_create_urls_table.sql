-- Migration 001: Create the urls table.
-- Run this against the target PostgreSQL database before starting the application.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS urls (
    id               TEXT        PRIMARY KEY,          -- UUID v4 stored as text
    short_code       TEXT        NOT NULL UNIQUE,      -- 7-char MD5 or custom alias
    long_url         TEXT        NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at       TIMESTAMPTZ,                      -- NULL means never expires
    click_count      BIGINT      NOT NULL DEFAULT 0,
    last_accessed_at TIMESTAMPTZ                       -- updated on every redirect
);

-- Index for the hot redirect path: lookup by short_code.
CREATE INDEX IF NOT EXISTS idx_urls_short_code ON urls (short_code);

-- Index for expiry sweeps (future background cleanup job).
CREATE INDEX IF NOT EXISTS idx_urls_expires_at ON urls (expires_at)
    WHERE expires_at IS NOT NULL;
