# GoURL — Production-Grade URL Shortener

> A full-stack URL shortener built with **Go**, **React**, **PostgreSQL**, **Redis**, and **Kafka** — containerized with Docker and designed with clean architecture principles.

---

## Table of Contents

- [Overview](#overview)
- [Live Demo & Screenshots](#live-demo--screenshots)
- [Key Features](#key-features)
- [Tech Stack](#tech-stack)
- [System Architecture](#system-architecture)
- [Project Structure](#project-structure)
- [Getting Started](#getting-started)
- [API Reference](#api-reference)
- [Database Schema](#database-schema)
- [Kafka Analytics Flow](#kafka-analytics-flow)
- [Redis Caching Strategy](#redis-caching-strategy)
- [Rate Limiting](#rate-limiting)
- [Environment Variables](#environment-variables)
- [Load Testing](#load-testing)
- [Development (Without Docker)](#development-without-docker)
- [Docker Commands](#docker-commands)
- [What I Learned & Design Decisions](#what-i-learned--design-decisions)

---

## Overview

GoURL started as a single `main.go` file with an in-memory map. It was progressively evolved into a **production-ready, full-stack application** to practice real-world backend engineering concepts — clean architecture, async event-driven processing, caching strategies, and containerized deployments.

**This project demonstrates:**
- Designing a layered, testable Go backend with dependency injection
- Using Redis as a cache-aside layer to reduce database load
- Decoupling analytics writes from the hot redirect path via Kafka
- Serving a React SPA through Nginx with reverse-proxy routing
- Orchestrating a 5-service Docker stack with health checks and proper startup ordering

---

## Key Features

| Feature | Details |
|---|---|
| **URL Shortening** | Generates a 7-character short code using MD5 hashing |
| **Custom Aliases** | User-defined vanity slugs (3–50 characters) |
| **Link Expiration** | Set links to expire in 1h / 24h / 7d / 30d |
| **Click Analytics** | Real-time click count, last accessed time, expiry info |
| **Redis Cache** | Cache-aside pattern; redirect resolution in ~1ms on cache hit |
| **Kafka Analytics** | Async click counting — never delays the 302 redirect |
| **Rate Limiting** | Per-IP: 60 requests/minute, returns `429` with retry info |
| **Health Check** | Live dependency check endpoint for PostgreSQL & Redis |
| **History Panel** | Last 10 shortened URLs persisted in localStorage |
| **Fully Dockerized** | One command to run the entire 5-service stack |

---

## Tech Stack

| Layer | Technology | Why |
|---|---|---|
| **Frontend** | React 18 + TypeScript + Vite + Tailwind CSS | Type-safe, fast HMR, utility-first styling |
| **Backend** | Go 1.21 + `chi` router | Fast, low memory, excellent concurrency primitives |
| **Database** | PostgreSQL 16 | ACID compliance, strong indexing, reliable at scale |
| **Cache** | Redis 7 | Sub-millisecond reads, TTL support, LRU eviction |
| **Messaging** | Apache Kafka 3.7 (KRaft, no Zookeeper) | Durable event log, decouples analytics from hot path |
| **Proxy** | Nginx 1.27 | Serves React SPA, reverse-proxies API calls |
| **Containers** | Docker + Docker Compose | Reproducible environments, single-command setup |

---

## System Architecture

```
Browser (localhost:3000)
        │
        ▼
  ┌─────────────────────┐
  │  Nginx (Frontend)   │  ← Serves React SPA (index.html)
  └─────────────────────┘
        │
        ├── /api/*      ──────────────────────────────────┐
        ├── /health     ──────────────────────────────────┤
        └── /{code}     ──────────────────────────────────┤
                                                          ▼
                                               ┌─────────────────┐
                                               │  Go API :8080   │
                                               └─────────────────┘
                                                    │      │      │
                                               ┌────┘  ┌───┘  ┌──┘
                                               ▼       ▼      ▼
                                          Postgres  Redis  Kafka
                                                            │
                                                            ▼
                                                  Analytics Consumer
                                                  (goroutine in app)
                                                            │
                                                            ▼
                                                  UPDATE click_count
                                                  in PostgreSQL
```

### Clean Architecture Layers

```
HTTP Request
     │
     ▼
 Handler        ← validates input, writes HTTP response
     │
     ▼
 Service        ← business logic, orchestrates dependencies
     │
     ▼
 Repository     ← PostgreSQL queries + Redis cache
     │
     ▼
 Database / Cache / Kafka
```

Each layer depends only on **interfaces** — not concrete types. Swapping PostgreSQL for another database requires only a new repository implementation; no business logic changes.

### Redirect Flow (Cache-Aside)

```
GET /{shortCode}
  │
  ├─▶ Check Redis
  │     ├── HIT  → 302 Redirect immediately (~1ms)
  │     └── MISS → Query PostgreSQL
  │                   → Store result in Redis (cache warming)
  │                   → 302 Redirect
  │
  └─▶ goroutine spawned (non-blocking):
        → Kafka producer publishes ClickEvent (async)
        → Consumer reads event
        → UPDATE click_count in PostgreSQL
```

---

## Project Structure

```
URLShortner/
├── backend/                          ← Go API (Clean Architecture)
│   ├── cmd/api/main.go               ← Application entrypoint
│   ├── config/config.go              ← Environment-based configuration
│   ├── internal/
│   │   ├── models/                   ← Domain structs (URL, ClickEvent)
│   │   ├── repository/               ← PostgreSQL + Redis data access
│   │   ├── service/                  ← Core business logic
│   │   ├── handler/                  ← HTTP handlers + chi router setup
│   │   └── middleware/               ← Logger, recovery, rate limiter
│   ├── kafka/                        ← Kafka producer + analytics consumer
│   ├── migrations/                   ← SQL schema (auto-runs on startup via go:embed)
│   └── Dockerfile
│
├── frontend/                         ← React SPA
│   ├── src/
│   │   ├── App.tsx                   ← Root component
│   │   ├── api.ts                    ← Typed API client
│   │   ├── types.ts                  ← TypeScript interfaces
│   │   ├── history.ts                ← localStorage persistence
│   │   └── components/
│   │       ├── ShortenForm.tsx       ← URL input + options form
│   │       ├── ResultCard.tsx        ← Short URL result display
│   │       ├── AnalyticsModal.tsx    ← Click analytics popup
│   │       ├── HistoryPanel.tsx      ← Recent URLs (last 10)
│   │       ├── CopyButton.tsx        ← Clipboard copy with feedback
│   │       └── HealthBadge.tsx       ← Live system status indicator
│   ├── nginx.conf                    ← Reverse proxy configuration
│   └── Dockerfile
│
├── load-tests/                       ← k6 load test scripts
│   ├── 01_cache_latency.js           ← Redis cache hit/miss benchmark
│   ├── 02_throughput.js              ← Max requests/sec test
│   ├── 03_rate_limiter.js            ← Rate limiting verification
│   ├── 04_create_url.js              ← URL creation under load
│   └── run_all.ps1                   ← Run all tests sequentially
│
├── docker-compose.yml                ← Full 5-service stack definition
├── .env.example                      ← Environment variable template
└── PROJECT.md                        ← Full technical documentation
```

---

## Getting Started

### Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) — **that's it**. No Go, Node, or databases required locally.

### One-Command Setup

```bash
# 1. Clone the repository
git clone https://github.com/VinayGadKerkar/Url_Shortner.git
cd Url_Shortner

# 2. Configure environment
cp backend/.env.example .env
# Open .env and set DB_PASSWORD to any value, e.g. DB_PASSWORD=postgres

# 3. Start the full stack
docker compose up -d
```

### Access the App

| Service | URL |
|---|---|
| **Frontend UI** | **http://localhost:3000** |
| Backend API | http://localhost:8080 |
| Health Check | http://localhost:8080/health |

> **Note:** On first run, Docker pulls images (~1–2 GB). Kafka takes ~15–20 seconds to be ready. If the frontend shows a red health badge initially, wait a moment and refresh.

---

## API Reference

### `POST /api/v1/shorten`
Create a short URL.

**Request Body**
```json
{
  "long_url": "https://example.com/some/very/long/path",
  "custom_alias": "my-link",        // optional, 3–50 chars
  "expires_in_hours": 24            // optional, 0 = never expires
}
```

**Response `201 Created`**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "short_code": "ab12cd3",
  "short_url": "http://localhost:8080/ab12cd3",
  "long_url": "https://example.com/some/very/long/path",
  "created_at": "2026-06-25T00:00:00Z",
  "expires_at": "2026-06-26T00:00:00Z"
}
```

**Error Codes**
| Code | Reason |
|---|---|
| `400` | Missing or invalid `long_url`, alias out of allowed range |
| `409` | Custom alias is already taken |

---

### `GET /{shortCode}`
Redirect to the original URL.

| Code | Meaning |
|---|---|
| `302` | Redirect to the original long URL |
| `404` | Short code not found |
| `410` | URL has expired |

---

### `GET /api/v1/analytics/{shortCode}`
Retrieve click analytics for a short URL.

**Response `200 OK`**
```json
{
  "short_code": "ab12cd3",
  "long_url": "https://example.com/some/very/long/path",
  "click_count": 42,
  "created_at": "2026-06-25T00:00:00Z",
  "expires_at": null,
  "last_accessed_at": "2026-06-25T12:00:00Z"
}
```

---

### `GET /health`
Live dependency check — used by the frontend health badge.

**Response `200 OK`**
```json
{
  "status": "ok",
  "components": {
    "postgres": "healthy",
    "redis": "healthy"
  }
}
```
Returns `503 Service Unavailable` with `"status": "degraded"` if any dependency is down.

---

## Database Schema

```sql
CREATE TABLE urls (
    id               TEXT        PRIMARY KEY,      -- UUID v4
    short_code       TEXT        NOT NULL UNIQUE,  -- 7-char MD5 hash or custom alias
    long_url         TEXT        NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at       TIMESTAMPTZ,                  -- NULL = never expires
    click_count      BIGINT      NOT NULL DEFAULT 0,
    last_accessed_at TIMESTAMPTZ
);

-- Indexed for fast redirect lookups
CREATE INDEX idx_urls_short_code ON urls (short_code);

-- Partial index — only indexes rows that actually have an expiry
CREATE INDEX idx_urls_expires_at ON urls (expires_at) WHERE expires_at IS NOT NULL;
```

> Migrations run automatically on every application startup using Go's `//go:embed` directive — no manual SQL steps required.

---

## Kafka Analytics Flow

Click counting is **fully decoupled** from the redirect path. The user receives their `302` redirect before any analytics write happens.

```
1. Browser → GET /{shortCode}
2. Go handler resolves URL (Redis or Postgres)
3. http.Redirect(302) sent immediately  ← user is already gone
4. goroutine spawned (fire-and-forget):
5.   Producer publishes ClickEvent JSON to "url-clicks" topic
6.   Analytics consumer goroutine reads the message
7.   PostgreSQL: UPDATE urls SET click_count = click_count + 1
```

**Why Kafka instead of a direct DB UPDATE?**
- A direct `UPDATE` on every redirect adds ~10ms of DB latency to the hot path
- Under high traffic, concurrent updates to the same row cause **lock contention**
- Kafka buffers the writes; the consumer processes them at its own pace
- The producer uses `Async=true` — the 302 is **never delayed** by Kafka availability
- This pattern scales to millions of redirects/day without saturating the database

---

## Redis Caching Strategy

**Pattern: Cache-Aside (Lazy Loading) with Cache Warming**

```
Redirect request arrives
         │
         ▼
   Check Redis (url:{shortCode})
         │
   ┌─────┴──────┐
   HIT          MISS
   │            │
   ▼            ▼
Return      Query PostgreSQL
immediately      │
(~1ms)      Store in Redis
             │
             ▼
            Return
```

**Implementation details:**
- Cache key: `url:{shortCode}`
- Default TTL: **1 hour** (configurable via `REDIS_CACHE_TTL_SECONDS`)
- For expiring URLs: TTL = `min(configured_ttl, time_until_expiry)` — Redis never serves a stale redirect past expiry
- **Cache warming:** New URLs are written to Redis immediately on creation — first redirect is always a cache hit
- **Eviction policy:** `allkeys-lru` — under memory pressure, least recently used entries are dropped; PostgreSQL is always the source of truth

---

## Rate Limiting

- **Scope:** Per client IP address
- **Limit:** 60 requests / minute (configurable in `cmd/api/main.go`)
- **Response on limit exceeded:**

```
HTTP 429 Too Many Requests
{
  "error": "rate limit exceeded",
  "retry_after_seconds": 60
}
```

- Current implementation: **in-memory per instance**
- For multi-replica deployments: swap to a Redis-backed store via `httprate.WithKeyFuncs` — no business logic changes required

---

## Environment Variables

Copy `backend/.env.example` to `.env` in the project root before starting.

| Variable | Default | Required | Description |
|---|---|---|---|
| `DB_PASSWORD` | — | ✅ Yes | PostgreSQL password |
| `DB_USER` | `postgres` | No | PostgreSQL username |
| `DB_NAME` | `urlshortener` | No | PostgreSQL database name |
| `DB_HOST` | `localhost` | No | PostgreSQL host (auto-set in Docker) |
| `DB_PORT` | `5432` | No | PostgreSQL port |
| `SERVER_PORT` | `8080` | No | Go API listening port |
| `BASE_URL` | `http://localhost:8080` | No | Base URL used in short URL responses |
| `REDIS_ADDR` | `localhost:6379` | No | Redis address (auto-set in Docker) |
| `REDIS_CACHE_TTL_SECONDS` | `3600` | No | Redis cache TTL (1 hour) |
| `KAFKA_BROKER` | `localhost:9092` | No | Kafka broker address (auto-set in Docker) |
| `KAFKA_TOPIC` | `url-clicks` | No | Kafka topic for click events |

> In Docker, `DB_HOST`, `REDIS_ADDR`, and `KAFKA_BROKER` are automatically overridden to Docker service names via `docker-compose.yml`.

---

## Load Testing

The `load-tests/` directory contains [k6](https://k6.io/) scripts to benchmark key system properties.

```bash
# Install k6 first: https://k6.io/docs/getting-started/installation/

# Run individual tests
k6 run load-tests/01_cache_latency.js    # Redis cache hit/miss latency
k6 run load-tests/02_throughput.js       # Maximum requests/sec
k6 run load-tests/03_rate_limiter.js     # Rate limiting behaviour
k6 run load-tests/04_create_url.js       # URL creation under load

# Run all tests (Windows PowerShell)
.\load-tests\run_all.ps1
```

---

## Development (Without Docker)

For local development with hot-reloading.

**Backend**
```bash
cd backend

# Ensure PostgreSQL, Redis, and Kafka are running locally
cp .env.example .env     # fill in your local connection details

go mod download
go run ./cmd/api
# API available at http://localhost:8080
```

**Frontend**
```bash
cd frontend

npm install
npm run dev
# UI available at http://localhost:3000
# /api and /health are proxied to localhost:8080 via Vite config
```

---

## Docker Commands

```bash
# Start all services in background
docker compose up -d

# Stop all services
docker compose down

# Stop and wipe all data (database, cache, Kafka)
docker compose down -v

# Rebuild after code changes
docker compose up -d --build           # rebuild everything
docker compose up -d --build app       # rebuild backend only
docker compose up -d --build frontend  # rebuild frontend only

# View logs
docker compose logs -f app             # backend logs
docker compose logs -f frontend        # nginx logs

# Check container status
docker compose ps
```

---

## What I Learned & Design Decisions

| Decision | Reasoning |
|---|---|
| **Clean Architecture** | Separating handler → service → repository enables independent testing and easy swapping of infrastructure (e.g., switch DBs without touching business logic) |
| **Kafka for analytics** | Avoids row-level locking on the `click_count` column under concurrent load; decouples the hot redirect path from write latency |
| **Cache-aside with warming** | Lazy loading reduces unnecessary cache writes; warming on creation ensures the first redirect hits cache |
| **KRaft Kafka (no Zookeeper)** | Simplifies the Docker stack by one service; Zookeeper-less mode is the production standard from Kafka 3.3+ |
| **Partial index on expires_at** | Only indexes rows that have an expiry — keeps the index small and fast for queries that filter on expiry |
| **`go:embed` for migrations** | Eliminates a runtime dependency on external SQL files; migrations are compiled into the binary and run automatically on startup |
| **MD5 for short code generation** | Preserved from the original `main.go` — deterministic, fast, and produces consistent 7-char codes for the same URL |

---

## Project Evolution

| Stage | What Changed |
|---|---|
| **v0 — Original** | Single `main.go`, in-memory map, stdlib only, port 8000 |
| **v1 — Refactor** | Clean Architecture layers, PostgreSQL persistence, Redis caching, Kafka analytics |
| **v2 — Frontend** | React + Vite + Tailwind SPA, served via Nginx with reverse-proxy routing |
| **v3 — Docker** | Full 5-container stack with health checks, dependency ordering, and one-command startup |

---

## Repository

**GitHub:** [VinayGadKerkar/Url_Shortner](https://github.com/VinayGadKerkar/Url_Shortner)

Full technical documentation: [PROJECT.md](./PROJECT.md)
