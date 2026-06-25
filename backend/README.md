# Backend — Go API

Production-grade URL shortener API built in Go using Clean Architecture.

## Structure

```
backend/
├── cmd/api/main.go         Entrypoint — wires all layers, starts HTTP server
├── config/config.go        Env-based configuration (all os.Getenv in one place)
├── internal/
│   ├── models/             Domain structs + request/response types + validation
│   ├── repository/         Interfaces + PostgreSQL and Redis implementations
│   ├── service/            Business logic (short code gen, cache-aside, analytics)
│   ├── handler/            HTTP handlers (thin — decode → service → encode)
│   └── middleware/         Request logger, panic recovery, IP rate limiter
├── kafka/
│   ├── producer.go         Publishes ClickEvent on every redirect
│   └── consumer.go         Reads events, increments click_count in PostgreSQL
├── migrations/
│   ├── 001_create_urls_table.sql   Schema with indexes
│   └── migrate.go          Embedded SQL runner (auto-runs on startup)
├── Dockerfile              Two-stage build → ~15MB Alpine image
├── .env.example            All supported environment variables with defaults
└── go.mod
```

## Running locally

```bash
# Requires: Go 1.21+, PostgreSQL, Redis, Kafka
cp .env.example .env
go run ./cmd/api
```

## Key design decisions

- **MD5 short codes** — first 7 chars of hex MD5, preserved from original implementation
- **Cache-aside** — Redis checked before every DB query; cache warmed on create
- **Async analytics** — Kafka decouples click counting from the redirect hot path
- **Interfaces** — `URLRepository` and `CacheRepository` are interfaces; swap the DB without touching service/handler code
- **Embedded migrations** — SQL files compiled into the binary via `//go:embed`; no migration tool needed
- **Graceful shutdown** — OS signal handling with configurable drain timeout

## Environment variables

See `.env.example` for the full list with descriptions.
