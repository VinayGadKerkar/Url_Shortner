# URL Shortener — Full Application Document

## Overview

A production-grade URL shortener built from scratch. Started as a single `main.go` file with an in-memory map. Evolved into a full-stack application with a Go backend, React frontend, PostgreSQL database, Redis cache, and Kafka analytics pipeline — all running in Docker.

---

## Project Structure

```
URLShortner/
├── backend/                        ← Go API
│   ├── cmd/api/main.go             ← Entrypoint
│   ├── config/config.go            ← Env-based config
│   ├── internal/
│   │   ├── models/                 ← Domain structs
│   │   ├── repository/             ← PostgreSQL + Redis
│   │   ├── service/                ← Business logic
│   │   ├── handler/                ← HTTP handlers + router
│   │   └── middleware/             ← Logger, recovery, rate limiter
│   ├── kafka/                      ← Producer + consumer
│   ├── migrations/                 ← SQL schema + runner
│   ├── Dockerfile
│   ├── docker-compose.yml          ← Full stack definition
│   └── .env
└── frontend/                       ← React + Vite + Tailwind
    ├── src/
    │   ├── App.tsx                 ← Root component
    │   ├── api.ts                  ← Backend API client
    │   ├── types.ts                ← TypeScript types
    │   ├── history.ts              ← localStorage helper
    │   └── components/
    │       ├── ShortenForm.tsx     ← URL input form
    │       ├── ResultCard.tsx      ← Short URL result
    │       ├── AnalyticsModal.tsx  ← Click analytics
    │       ├── HistoryPanel.tsx    ← Recent URLs
    │       ├── CopyButton.tsx      ← Clipboard copy
    │       └── HealthBadge.tsx     ← Live system status
    ├── nginx.conf                  ← Reverse proxy config
    └── Dockerfile
```

---

## Architecture

```
Browser (localhost:3000)
        │
        ▼
  Nginx (frontend container)
        │
        ├── /api/*     ──────────────────────────────┐
        ├── /health    ──────────────────────────────┤
        ├── /{code}    ──────────────────────────────┤
        │                                            ▼
        │                                   Go API (port 8080)
        │                                        │
        └── /*  →  React SPA (index.html)        ├── PostgreSQL
                                                 ├── Redis (cache)
                                                 └── Kafka (analytics)
                                                        │
                                                        ▼
                                              Analytics Consumer
                                              (goroutine in app)
                                                        │
                                                        ▼
                                              PostgreSQL click_count
```

### Clean Architecture Layers

```
Handler  →  Service  →  Repository  →  Database
  ↑              ↓
HTTP           Cache (Redis)
               Kafka (events)
```

Each layer only depends on the layer below it. The service depends on interfaces, not concrete types — swapping PostgreSQL for another DB requires only a new repository implementation.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Frontend | React 18, TypeScript, Vite, Tailwind CSS |
| Backend | Go 1.21, chi router |
| Database | PostgreSQL 16 |
| Cache | Redis 7 |
| Messaging | Apache Kafka 3.7 (KRaft, no Zookeeper) |
| Proxy | Nginx 1.27 |
| Container | Docker + Docker Compose |

---

## API Reference

### POST /api/v1/shorten
Create a short URL.

**Request**
```json
{
  "long_url": "https://example.com/some/long/path",
  "custom_alias": "my-link",
  "expires_in_hours": 24
}
```

**Response 201**
```json
{
  "id": "uuid",
  "short_code": "ab12cd3",
  "short_url": "http://localhost:8080/ab12cd3",
  "long_url": "https://example.com/some/long/path",
  "created_at": "2026-06-25T00:00:00Z",
  "expires_at": "2026-06-26T00:00:00Z"
}
```

**Errors**
- `400` — missing/invalid long_url or alias out of range
- `409` — custom alias already taken

---

### GET /{shortCode}
Redirect to the original URL.

- `302` — redirect to long URL
- `404` — short code not found
- `410` — URL has expired

---

### GET /api/v1/analytics/{shortCode}
Get click analytics for a short URL.

**Response 200**
```json
{
  "short_code": "ab12cd3",
  "long_url": "https://example.com/...",
  "click_count": 42,
  "created_at": "2026-06-25T00:00:00Z",
  "expires_at": null,
  "last_accessed_at": "2026-06-25T12:00:00Z"
}
```

---

### GET /health
Live dependency check.

**Response 200**
```json
{
  "status": "ok",
  "components": {
    "postgres": "healthy",
    "redis": "healthy"
  }
}
```
Returns `503` with `"status": "degraded"` if any dependency is down.

---

## Database Schema

```sql
CREATE TABLE urls (
    id               TEXT        PRIMARY KEY,      -- UUID v4
    short_code       TEXT        NOT NULL UNIQUE,  -- 7-char MD5 or custom alias
    long_url         TEXT        NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at       TIMESTAMPTZ,                  -- NULL = never expires
    click_count      BIGINT      NOT NULL DEFAULT 0,
    last_accessed_at TIMESTAMPTZ
);

CREATE INDEX idx_urls_short_code ON urls (short_code);
CREATE INDEX idx_urls_expires_at ON urls (expires_at) WHERE expires_at IS NOT NULL;
```

Migrations run automatically on every app startup via `//go:embed` — no manual SQL steps needed.

---

## Redis Cache Strategy

Pattern: **Cache-Aside (Lazy Loading)**

```
Redirect request
      │
      ▼
Check Redis → HIT  → return immediately (~1ms)
      │
      └── MISS → query PostgreSQL → store in Redis → return
```

- Cache key: `url:{shortCode}`
- Default TTL: 1 hour (configurable via `REDIS_CACHE_TTL_SECONDS`)
- If a URL has an `expires_at`, the TTL is set to `min(configured_ttl, time_until_expiry)` — Redis never serves a redirect past expiry
- New URLs are written to Redis immediately on creation (cache warming)
- Eviction policy: `allkeys-lru` — under memory pressure, least recently used URLs are evicted. DB is always the source of truth.

---

## Kafka Analytics Flow

Click counting is fully decoupled from the redirect path:

```
1. Browser requests /{shortCode}
2. Go handler resolves URL (Redis/Postgres)
3. http.Redirect(302) sent to browser  ← user is already redirected
4. goroutine spawned →
5.   Kafka producer publishes ClickEvent JSON to "url-clicks" topic
6.   Analytics consumer reads message
7.   PostgreSQL: UPDATE SET click_count = click_count + 1
```

**Why Kafka instead of a direct DB write?**
- A direct `UPDATE` on every redirect adds ~10ms DB latency to the hot path
- Under high traffic, concurrent updates to the same row cause lock contention
- Kafka buffers the writes; the consumer processes them at its own pace
- The producer uses `Async=true` so the 302 is never delayed by Kafka

---

## Rate Limiting

- Per-IP rate limiting: 60 requests/minute (configurable in `cmd/api/main.go`)
- On limit exceeded, returns:
```json
HTTP 429
{"error": "rate limit exceeded", "retry_after_seconds": 60}
```
- Current implementation: in-memory per instance
- For multi-replica deployments: swap to Redis-backed store via `httprate.WithKeyFuncs`

---

## Frontend Features

| Feature | Description |
|---|---|
| Shorten form | Paste any URL, get a short link |
| Custom alias | Optional vanity slug (3–50 chars) |
| Expiry | Set link to expire in 1h / 24h / 7d / 30d |
| Copy button | One-click clipboard copy, turns green on success |
| Result card | Shows short URL, original URL, created time, expiry |
| Analytics modal | Click count, created/last accessed/expiry stats |
| History panel | Last 10 shortened URLs, persisted in localStorage |
| Health badge | Live green/red dot in navbar, polls /health every 30s |

---

## Running the Application

### Prerequisites
Docker Desktop installed and running. Nothing else needed.

### Start
```bash
cd backend
cp .env.example .env      # set DB_PASSWORD=postgres (or any value)
docker compose up -d
```

### Access
| Service | URL |
|---|---|
| **Frontend UI** | **http://localhost:3000** |
| Backend API | http://localhost:8080 |
| Health check | http://localhost:8080/health |

### Common commands
```bash
# View logs
docker compose logs -f app       # backend
docker compose logs -f frontend  # nginx + react

# Stop everything
docker compose down

# Full reset (wipe database)
docker compose down -v

# Rebuild after code changes
docker compose up -d --build
docker compose up -d --build frontend   # rebuild only frontend
docker compose up -d --build app        # rebuild only backend
```

---

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `SERVER_PORT` | `8080` | Go API port |
| `BASE_URL` | `http://localhost:8080` | Used in short URL responses |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `postgres` | PostgreSQL user |
| `DB_PASSWORD` | **required** | PostgreSQL password |
| `DB_NAME` | `urlshortener` | PostgreSQL database name |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_CACHE_TTL_SECONDS` | `3600` | Cache TTL (1 hour) |
| `KAFKA_BROKER` | `localhost:9092` | Kafka broker address |
| `KAFKA_TOPIC` | `url-clicks` | Kafka topic for click events |

In Docker, `DB_HOST`, `REDIS_ADDR`, and `KAFKA_BROKER` are automatically overridden to Docker service names by the `environment` block in `docker-compose.yml`.

---

## What Was Built — Evolution

| Stage | What changed |
|---|---|
| Original | Single `main.go`, in-memory map, stdlib only, port 8000 |
| Refactor | Clean Architecture layers, PostgreSQL, Redis, Kafka, Docker |
| Frontend | React + Vite + Tailwind, served via Nginx |
| Docker | Full 5-container stack, health checks, dependency ordering |

The MD5-based short code generation from the original `main.go` was preserved throughout — `generateShortCode()` in `service/url_service.go` is identical logic.
