# Snip — URL Shortener

A production-grade URL shortener built with Go, React, PostgreSQL, Redis, and Kafka. Started as a single `main.go` with an in-memory map — evolved into a full-stack application with clean architecture, async analytics, and a complete Docker setup.

---

## Quick Start

The only prerequisite is [Docker Desktop](https://www.docker.com/products/docker-desktop/).

```bash
# 1. Clone
git clone <repo-url>
cd URLShortner

# 2. Configure
cp backend/.env.example .env
# Edit .env — set DB_PASSWORD to any value e.g. DB_PASSWORD=postgres

# 3. Start everything
docker compose up -d

# 4. Open
# Frontend UI  → http://localhost:3000
# Backend API  → http://localhost:8080
# Health check → http://localhost:8080/health
```

---

## Stack

| Layer | Technology |
|---|---|
| Frontend | React 18 + TypeScript + Vite + Tailwind CSS |
| Backend | Go 1.21 + chi router |
| Database | PostgreSQL 16 |
| Cache | Redis 7 (cache-aside pattern) |
| Messaging | Apache Kafka 3.7 (KRaft, no Zookeeper) |
| Proxy | Nginx 1.27 |
| Containers | Docker + Docker Compose |

---

## Architecture

```
Browser :3000
    │
    ▼
Nginx (frontend container)
    ├── /api/*   ──┐
    ├── /health  ──┤──▶  Go API :8080
    ├── /{code}  ──┘         │
    │                        ├── PostgreSQL (storage)
    └── /*  ──▶ React SPA    ├── Redis      (cache)
                             └── Kafka      (click events)
                                                │
                                         Analytics Consumer
                                         (goroutine in app)
                                                │
                                         click_count in DB
```

### Redirect flow (cache-aside)
```
GET /{shortCode}
  → check Redis      (hit: ~1ms, redirect immediately)
  → miss: query DB   (populate Redis, redirect)
  → goroutine: publish ClickEvent to Kafka
  → consumer: UPDATE click_count in PostgreSQL
```

---

## Project Layout

```
URLShortner/
├── backend/          Go API — Clean Architecture
│   ├── cmd/api/      Entrypoint
│   ├── config/       Environment config
│   ├── internal/
│   │   ├── models/       Domain structs
│   │   ├── repository/   PostgreSQL + Redis
│   │   ├── service/      Business logic
│   │   ├── handler/      HTTP handlers + router
│   │   └── middleware/   Logging, recovery, rate limiting
│   ├── kafka/        Producer + consumer
│   ├── migrations/   SQL schema (auto-runs on startup)
│   └── Dockerfile
├── frontend/         React SPA
│   ├── src/
│   │   ├── components/   UI components
│   │   ├── api.ts        Backend API client
│   │   └── types.ts      TypeScript types
│   ├── nginx.conf    Reverse proxy config
│   └── Dockerfile
├── docker-compose.yml
├── .env.example      ← copy to .env and fill in
└── PROJECT.md        Full technical documentation
```

---

## API

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/v1/shorten` | Create a short URL |
| `GET` | `/{shortCode}` | Redirect to original URL |
| `GET` | `/api/v1/analytics/{shortCode}` | Click analytics |
| `GET` | `/health` | System health check |

See [PROJECT.md](./PROJECT.md) for full API documentation with request/response shapes.

---

## Docker Commands

```bash
docker compose up -d              # start all services
docker compose down               # stop all services
docker compose down -v            # stop + wipe all data
docker compose up -d --build      # rebuild after code changes
docker compose up -d --build app  # rebuild only backend
docker compose up -d --build frontend  # rebuild only frontend
docker compose logs -f app        # tail backend logs
docker compose logs -f frontend   # tail nginx logs
docker compose ps                 # check container status
```

---

## Environment Variables

Copy `backend/.env.example` to `.env` in the root and set these:

| Variable | Required | Default | Description |
|---|---|---|---|
| `DB_PASSWORD` | ✅ | — | PostgreSQL password |
| `DB_USER` | | `postgres` | PostgreSQL user |
| `DB_NAME` | | `urlshortener` | PostgreSQL database |
| `SERVER_PORT` | | `8080` | Go API port |
| `BASE_URL` | | `http://localhost:8080` | Used in short URL responses |
| `REDIS_CACHE_TTL_SECONDS` | | `3600` | Cache TTL (1 hour) |
| `KAFKA_TOPIC` | | `url-clicks` | Kafka topic name |

---

## Development (without Docker)

**Backend**
```bash
cd backend
cp .env.example .env   # fill in connection details for local services
go mod download
go run ./cmd/api
```

**Frontend**
```bash
cd frontend
npm install
npm run dev            # starts on http://localhost:3000
                       # proxies /api and /health to localhost:8080
```
