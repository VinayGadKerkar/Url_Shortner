# Codebase Guide — URL Shortener

A file-by-file explanation of every source file in the project, with the key code snippet from each one and a plain-English explanation of what it does and why it exists.

---

## Project Structure

```
URLShortner/
├── cmd/api/main.go                      ← Application entrypoint
├── config/config.go                     ← Environment-based configuration
├── internal/
│   ├── models/
│   │   ├── url.go                       ← Core domain structs
│   │   └── event.go                     ← Kafka message payload
│   ├── repository/
│   │   ├── interfaces.go                ← Repository contracts (interfaces)
│   │   ├── postgres.go                  ← PostgreSQL implementation
│   │   └── redis.go                     ← Redis cache implementation
│   ├── service/
│   │   └── url_service.go               ← Business logic
│   ├── handler/
│   │   ├── router.go                    ← Route wiring
│   │   ├── url_handler.go               ← HTTP handlers for URL endpoints
│   │   ├── health_handler.go            ← Health check endpoint
│   │   └── response.go                  ← JSON response helpers
│   └── middleware/
│       ├── logger.go                    ← Request logging
│       ├── recovery.go                  ← Panic recovery
│       └── ratelimit.go                 ← IP rate limiting
├── kafka/
│   ├── producer.go                      ← Kafka click event publisher
│   └── consumer.go                      ← Analytics consumer
├── migrations/
│   ├── 001_create_urls_table.sql        ← Database schema
│   └── migrate.go                       ← Migration runner
├── Dockerfile
├── docker-compose.yml
└── .env.example
```

---

## Layer Overview

The code is organised in **Clean Architecture** layers. Each layer only talks to the layer below it — never above.

```
HTTP Request
     ↓
  Handler      ← decodes request, calls service, encodes response
     ↓
  Service      ← business logic, orchestrates repo + cache
     ↓
Repository     ← talks to PostgreSQL and Redis
     ↓
 Database / Cache / Kafka
```

---

## `cmd/api/main.go` — Application Entrypoint

**What it does:** Boots the entire application. It is the only file that knows about every component — it creates them all, wires them together, and starts the HTTP server. Think of it as the assembly line.

**Key snippet — wiring everything together:**
```go
// Repositories
urlRepo  := repository.NewPostgresURLRepository(pool)
urlCache := repository.NewRedisURLCache(redisClient, cfg.Redis.CacheTTL)

// Service gets the repo and cache injected
urlSvc := service.NewURLService(urlRepo, urlCache, logger, cfg.Server.BaseURL)

// Handlers get the service injected
urlHandler    := handler.NewURLHandler(urlSvc, producer, logger)
healthHandler := handler.NewHealthHandler(urlCache, pool.Ping, logger)
```

**Key snippet — graceful shutdown:**
```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
<-ctx.Done()  // blocks here until Ctrl+C or SIGTERM

shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
srv.Shutdown(shutdownCtx)  // drains in-flight requests before exiting
```

**Key snippet — PostgreSQL retry loop:**
```go
for i := 1; i <= maxRetries; i++ {
    pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
    if err == nil {
        if pingErr := pool.Ping(context.Background()); pingErr == nil {
            return pool, nil  // connected
        }
    }
    time.Sleep(retryDelay)  // wait and retry — needed because Docker starts
}                           // the app before Postgres is fully ready
```

---

## `config/config.go` — Configuration

**What it does:** Reads every configurable value from environment variables and returns a single `Config` struct. No other file calls `os.Getenv` — all config is centralised here.

**Key snippet — typed config structs:**
```go
type Config struct {
    Server   ServerConfig
    Database DatabaseConfig
    Redis    RedisConfig
    Kafka    KafkaConfig
}
```

**Key snippet — loading with defaults:**
```go
Server: ServerConfig{
    Port:    getEnv("SERVER_PORT", "8080"),   // default 8080
    BaseURL: getEnv("BASE_URL", "http://localhost:8080"),
},
Database: DatabaseConfig{
    Host:     getEnv("DB_HOST", "localhost"),
    Password: getEnv("DB_PASSWORD", ""),      // required — no default
},
```

**Key snippet — DSN builder:**
```go
func (d DatabaseConfig) DSN() string {
    return fmt.Sprintf(
        "host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
        d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode,
    )
}
```

`DB_PASSWORD` is the only mandatory variable. If it's missing, `Load()` returns an error and the app refuses to start.

---

## `internal/models/url.go` — Domain Models

**What it does:** Defines the core data shapes used across every layer. These are plain Go structs with zero external dependencies — no database tags, no framework imports.

**Key snippet — the URL domain model:**
```go
type URL struct {
    ID         string     `json:"id"`
    ShortCode  string     `json:"short_code"`
    LongURL    string     `json:"long_url"`
    CreatedAt  time.Time  `json:"created_at"`
    ExpiresAt  *time.Time `json:"expires_at,omitempty"` // nil = never expires
    ClickCount int64      `json:"click_count"`
}

func (u *URL) IsExpired() bool {
    if u.ExpiresAt == nil {
        return false
    }
    return time.Now().After(*u.ExpiresAt)
}
```

**Key snippet — request validation:**
```go
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
    return nil
}
```

Validation lives on the model, not in the handler or service — this keeps the rules close to the data they describe.

---

## `internal/models/event.go` — Kafka Event Payload

**What it does:** Defines the `ClickEvent` struct that is serialised to JSON and published to Kafka every time a redirect happens.

**Key snippet:**
```go
type ClickEvent struct {
    ShortCode  string    `json:"short_code"`
    AccessedAt time.Time `json:"accessed_at"`
    IPAddress  string    `json:"ip_address"`
    UserAgent  string    `json:"user_agent"`
}
```

This struct is the contract between the producer (handler) and the consumer (analytics worker). Both sides import it from this single file — there is no duplication.

---

## `internal/repository/interfaces.go` — Repository Contracts

**What it does:** Declares the two interfaces that define what a storage backend must be able to do. The service layer only ever talks to these interfaces — it never imports `postgres.go` or `redis.go` directly.

**Key snippet:**
```go
type URLRepository interface {
    Create(ctx context.Context, url *models.URL) (*models.URL, error)
    GetByShortCode(ctx context.Context, shortCode string) (*models.URL, error)
    IncrementClickCount(ctx context.Context, shortCode string) error
    ShortCodeExists(ctx context.Context, shortCode string) (bool, error)
}

type CacheRepository interface {
    Set(ctx context.Context, shortCode string, url *models.URL) error
    Get(ctx context.Context, shortCode string) (*models.URL, error)
    Delete(ctx context.Context, shortCode string) error
    Ping(ctx context.Context) error
}
```

Because the service depends on interfaces rather than concrete types, you could swap PostgreSQL for DynamoDB or Redis for Memcached by writing a new struct that satisfies the interface — without touching the service or handler code.

---

## `internal/repository/postgres.go` — PostgreSQL Implementation

**What it does:** Implements `URLRepository` using a PostgreSQL connection pool (`pgxpool`). All raw SQL lives here and nowhere else.

**Key snippet — creating a URL:**
```go
func (r *PostgresURLRepository) Create(ctx context.Context, url *models.URL) (*models.URL, error) {
    query := `
        INSERT INTO urls (id, short_code, long_url, created_at, expires_at, click_count)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id, short_code, long_url, created_at, expires_at, click_count
    `
    row := r.pool.QueryRow(ctx, query,
        url.ID, url.ShortCode, url.LongURL,
        url.CreatedAt, url.ExpiresAt, url.ClickCount,
    )
    // RETURNING lets us read back the saved row in one round trip
    var saved models.URL
    row.Scan(&saved.ID, &saved.ShortCode, &saved.LongURL,
             &saved.CreatedAt, &saved.ExpiresAt, &saved.ClickCount)
    return &saved, nil
}
```

**Key snippet — incrementing click count atomically:**
```go
func (r *PostgresURLRepository) IncrementClickCount(ctx context.Context, shortCode string) error {
    query := `
        UPDATE urls
        SET click_count     = click_count + 1,
            last_accessed_at = $1
        WHERE short_code = $2
    `
    ct, err := r.pool.Exec(ctx, query, time.Now().UTC(), shortCode)
    if ct.RowsAffected() == 0 {
        return ErrNotFound
    }
    return err
}
```

`click_count + 1` happens inside PostgreSQL — this makes the increment safe under concurrent requests without needing application-level locks.

---

## `internal/repository/redis.go` — Redis Cache Implementation

**What it does:** Implements `CacheRepository` using Redis. Stores serialised URL structs as JSON strings with a TTL. This is the cache-aside layer that makes redirects fast.

**Key snippet — storing a URL with smart TTL:**
```go
func (c *RedisURLCache) Set(ctx context.Context, shortCode string, url *models.URL) error {
    data, _ := json.Marshal(url)

    ttl := c.ttl  // default TTL from config (e.g. 1 hour)
    if url.ExpiresAt != nil {
        remaining := time.Until(*url.ExpiresAt)
        if remaining < ttl {
            ttl = remaining  // don't cache past the URL's own expiry
        }
    }

    return c.client.Set(ctx, "url:"+shortCode, data, ttl).Err()
}
```

**Key snippet — cache miss detection:**
```go
func (c *RedisURLCache) Get(ctx context.Context, shortCode string) (*models.URL, error) {
    data, err := c.client.Get(ctx, "url:"+shortCode).Bytes()
    if errors.Is(err, redis.Nil) {
        return nil, nil  // nil, nil = cache miss (not an error)
    }
    var url models.URL
    json.Unmarshal(data, &url)
    return &url, nil
}
```

Returning `nil, nil` on a cache miss is an intentional convention — the service checks `if cached != nil` and falls through to the DB lookup.

---

## `internal/service/url_service.go` — Business Logic

**What it does:** Contains all the business rules. The handlers call the service; the service calls the repositories. This is the most important layer — it owns the logic that makes the application correct.

**Key snippet — short code generation (preserved from original `main.go`):**
```go
func generateShortCode(input string) string {
    hasher := md5.New()
    hasher.Write([]byte(input))
    return hex.EncodeToString(hasher.Sum(nil))[:7]  // first 7 chars of MD5 hex
}
```

**Key snippet — cache-aside redirect flow:**
```go
func (s *URLService) Resolve(ctx context.Context, shortCode string) (*models.URL, error) {
    // 1. Check Redis first
    cached, _ := s.cache.Get(ctx, shortCode)
    if cached != nil {
        if cached.IsExpired() { return nil, ErrURLExpired }
        return cached, nil  // cache hit — return immediately
    }

    // 2. Cache miss — query PostgreSQL
    url, err := s.repo.GetByShortCode(ctx, shortCode)
    if err != nil { return nil, err }

    if url.IsExpired() { return nil, ErrURLExpired }

    // 3. Repopulate cache for next request
    s.cache.Set(ctx, url.ShortCode, url)

    return url, nil
}
```

**Key snippet — collision-safe short code creation:**
```go
shortCode = generateShortCode(req.LongURL)

exists, _ := s.repo.ShortCodeExists(ctx, shortCode)
if exists {
    // Append a UUID nonce to produce a different hash and retry once
    shortCode = generateShortCode(req.LongURL + uuid.New().String())
}
```

---

## `internal/handler/url_handler.go` — URL HTTP Handlers

**What it does:** Translates HTTP requests into service calls and HTTP responses. Handlers are intentionally thin — they decode, delegate, encode. No business logic lives here.

**Key snippet — creating a short URL:**
```go
func (h *URLHandler) CreateShortURL(w http.ResponseWriter, r *http.Request) {
    var req models.CreateURLRequest
    json.NewDecoder(r.Body).Decode(&req)  // decode JSON body

    resp, err := h.svc.CreateShortURL(r.Context(), &req)  // delegate to service
    if err != nil {
        // map service errors to HTTP status codes
        switch {
        case errors.Is(err, service.ErrShortCodeConflict):
            writeError(w, http.StatusConflict, "custom alias already in use")
        default:
            writeError(w, http.StatusInternalServerError, "failed to create short URL")
        }
        return
    }
    writeJSON(w, http.StatusCreated, resp)
}
```

**Key snippet — redirect with async Kafka event:**
```go
func (h *URLHandler) Redirect(w http.ResponseWriter, r *http.Request) {
    shortCode := chi.URLParam(r, "shortCode")
    url, err := h.svc.Resolve(r.Context(), shortCode)
    // ... error handling ...

    // Fire-and-forget: publish click event AFTER sending the redirect.
    // A separate goroutine is used so Kafka latency never delays the 302.
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        h.producer.PublishClickEvent(ctx, models.ClickEvent{
            ShortCode:  shortCode,
            AccessedAt: time.Now().UTC(),
        })
    }()

    http.Redirect(w, r, url.LongURL, http.StatusFound)  // 302 sent first
}
```

---

## `internal/handler/health_handler.go` — Health Check

**What it does:** Exposes `GET /health` and `HEAD /health`. It actively pings both PostgreSQL and Redis and reports their status. Used by Docker to decide if the container is healthy, and by load balancers to decide if the instance should receive traffic.

**Key snippet:**
```go
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
    defer cancel()

    components := make(map[string]string)
    healthy := true

    if err := h.dbPing(ctx); err != nil {
        components["postgres"] = "unhealthy"
        healthy = false
    } else {
        components["postgres"] = "healthy"
    }

    if err := h.cache.Ping(ctx); err != nil {
        components["redis"] = "unhealthy"
        healthy = false
    } else {
        components["redis"] = "healthy"
    }

    if healthy {
        writeJSON(w, http.StatusOK, ...)        // 200 OK
    } else {
        writeJSON(w, http.StatusServiceUnavailable, ...)  // 503
    }
}
```

The 3-second timeout ensures the health check never hangs if a dependency is slow — it fails fast.

---

## `internal/handler/router.go` — Route Wiring

**What it does:** Creates the chi router, attaches global middleware, and maps every URL path to its handler function. This is the single source of truth for what routes exist.

**Key snippet:**
```go
func NewRouter(deps RouterDeps) http.Handler {
    r := chi.NewRouter()

    // Global middleware — runs on every request in this order
    r.Use(chimiddleware.RequestID)   // adds X-Request-Id header
    r.Use(chimiddleware.RealIP)      // reads X-Forwarded-For
    r.Use(middleware.Recovery(...))  // catches panics
    r.Use(middleware.RequestLogger(...))
    r.Use(middleware.RateLimiterWithResponse(deps.RequestsPerMinute))

    r.Get("/health", deps.HealthHandler.Health)
    r.Head("/health", deps.HealthHandler.Health)  // for Docker wget probe

    r.Get("/{shortCode}", deps.URLHandler.Redirect)

    r.Route("/api/v1", func(r chi.Router) {
        r.Post("/shorten", deps.URLHandler.CreateShortURL)
        r.Get("/analytics/{shortCode}", deps.URLHandler.GetAnalytics)
    })

    return r
}
```

---

## `internal/handler/response.go` — JSON Helpers

**What it does:** Two tiny helper functions used by every handler to write consistent JSON responses. Centralising these means every endpoint always sets `Content-Type: application/json` and always encodes errors in the same shape.

**Key snippet:**
```go
func writeJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
    writeJSON(w, status, map[string]string{"error": message})
}
// Every error response looks like: {"error": "some message"}
```

---

## `internal/middleware/logger.go` — Request Logging

**What it does:** Wraps every HTTP handler to log the method, path, status code, and latency of every request using structured JSON (via zap). Docker health check probes are suppressed to keep logs clean.

**Key snippet — wrapping the ResponseWriter to capture status:**
```go
type responseWriter struct {
    http.ResponseWriter
    status int
}
func (rw *responseWriter) WriteHeader(code int) {
    rw.status = code                     // capture it
    rw.ResponseWriter.WriteHeader(code)  // then forward it
}
```

**Key snippet — suppressing health probe noise:**
```go
isHealthProbe := r.URL.Path == "/health" &&
    (r.Method == http.MethodHead || r.Method == http.MethodGet) &&
    r.UserAgent()[:4] == "Wget"
if isHealthProbe {
    return  // skip logging the Docker health check every 15 seconds
}
```

---

## `internal/middleware/recovery.go` — Panic Recovery

**What it does:** Wraps every request in a `defer/recover`. If any handler panics (e.g., nil pointer dereference), this middleware catches it, logs the full stack trace, and returns a `500 Internal Server Error` — keeping the server running instead of crashing.

**Key snippet:**
```go
func Recovery(logger *zap.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            defer func() {
                if rec := recover(); rec != nil {
                    logger.Error("panic recovered",
                        zap.Any("panic", rec),
                        zap.ByteString("stack", debug.Stack()),  // full goroutine stack
                    )
                    http.Error(w, "internal server error", http.StatusInternalServerError)
                }
            }()
            next.ServeHTTP(w, r)
        })
    }
}
```

---

## `internal/middleware/ratelimit.go` — Rate Limiting

**What it does:** Limits each IP address to a configurable number of requests per minute. If exceeded, returns `429 Too Many Requests` with a JSON body and a `Retry-After` header.

**Key snippet:**
```go
func RateLimiterWithResponse(requestsPerMinute int) func(http.Handler) http.Handler {
    return httprate.Limit(
        requestsPerMinute,
        time.Minute,
        httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Content-Type", "application/json")
            w.Header().Set("Retry-After", "60")
            w.WriteHeader(http.StatusTooManyRequests)
            w.Write([]byte(`{"error":"rate limit exceeded","retry_after_seconds":60}`))
        }),
    )
}
```

Currently uses an in-memory counter per instance. For multi-replica deployments, this can be backed by Redis by swapping the `httprate.Limit` call to use `httprate.WithKeyFuncs` and a Redis store.

---

## `kafka/producer.go` — Kafka Click Event Publisher

**What it does:** Wraps a `kafka-go` Writer and publishes a `ClickEvent` JSON message to the `url-clicks` topic every time a redirect happens. Configured as async so the 302 redirect is sent to the client before waiting for the broker acknowledgement.

**Key snippet — async writer with error logging:**
```go
writer := &kafkago.Writer{
    Addr:         kafkago.TCP(brokers...),
    Topic:        topic,
    Async:        true,           // returns immediately, ack happens in background
    RequiredAcks: kafkago.RequireOne,
    ErrorLogger:  &zapErrorLogger{logger: logger},  // surfaces async failures in logs
}
```

**Key snippet — publishing a message:**
```go
func (p *Producer) PublishClickEvent(ctx context.Context, event models.ClickEvent) error {
    data, _ := json.Marshal(event)

    msg := kafkago.Message{
        Key:   []byte(event.ShortCode),  // keyed by short code for ordering
        Value: data,
        Time:  event.AccessedAt,
    }

    return p.writer.WriteMessages(ctx, msg)
}
```

Messages are keyed by `ShortCode` so all clicks for the same URL go to the same partition — this guarantees ordered processing for that URL.

---

## `kafka/consumer.go` — Analytics Consumer

**What it does:** Reads `ClickEvent` messages from the `url-clicks` Kafka topic and calls `IncrementClickCount` on the PostgreSQL repository for each one. Runs as a long-lived goroutine started at application boot.

**Key snippet — the consume loop:**
```go
func (c *Consumer) Start(ctx context.Context) {
    for {
        msg, err := c.reader.ReadMessage(ctx)
        if err != nil {
            if ctx.Err() != nil {
                return  // graceful shutdown — context was cancelled
            }
            // transient error — back off and retry
            time.Sleep(2 * time.Second)
            continue
        }
        c.handleMessage(ctx, msg)
    }
}
```

**Key snippet — processing a message:**
```go
func (c *Consumer) handleMessage(ctx context.Context, msg kafkago.Message) {
    var event models.ClickEvent
    json.Unmarshal(msg.Value, &event)

    // This is the only DB write the consumer does — increment the counter
    c.repo.IncrementClickCount(ctx, event.ShortCode)
}
```

`StartOffset: kafkago.FirstOffset` means a brand-new consumer group starts reading from the oldest available message — so no clicks are lost even if the consumer starts after the producer.

---

## `migrations/001_create_urls_table.sql` — Database Schema

**What it does:** Defines the `urls` table that all URL data is stored in. It is safe to run multiple times because every statement uses `IF NOT EXISTS`.

**Key snippet:**
```sql
CREATE TABLE IF NOT EXISTS urls (
    id               TEXT        PRIMARY KEY,   -- UUID v4
    short_code       TEXT        NOT NULL UNIQUE,
    long_url         TEXT        NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at       TIMESTAMPTZ,               -- NULL = never expires
    click_count      BIGINT      NOT NULL DEFAULT 0,
    last_accessed_at TIMESTAMPTZ
);

-- Hot path index: every redirect does a WHERE short_code = $1 lookup
CREATE INDEX IF NOT EXISTS idx_urls_short_code ON urls (short_code);
```

The `short_code` index is the most important performance decision in the schema — without it, every redirect would be a full table scan.

---

## `migrations/migrate.go` — Migration Runner

**What it does:** Reads all `.sql` files embedded at compile time via `//go:embed` and executes them in filename order against the database. Called automatically on every application startup.

**Key snippet:**
```go
//go:embed *.sql
var sqlFiles embed.FS  // SQL files are baked into the binary at compile time

func Run(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) error {
    entries, _ := fs.ReadDir(sqlFiles, ".")

    // Sort ensures 001_... runs before 002_... before 003_...
    sort.Strings(files)

    for _, name := range files {
        content, _ := sqlFiles.ReadFile(name)
        pool.Exec(ctx, string(content))  // execute the SQL
    }
}
```

Because the SQL files use `IF NOT EXISTS`, running the migrations twice is safe — they are idempotent.

---

## `docker-compose.yml` — Full Stack Definition

**What it does:** Defines all four services (Kafka, PostgreSQL, Redis, Go API), their environment variables, health checks, and dependency order. A developer runs `docker compose up -d` and the entire stack starts correctly without installing anything locally.

**Key snippet — dependency ordering:**
```yaml
app:
  depends_on:
    postgres:
      condition: service_healthy  # waits for pg_isready to pass
    redis:
      condition: service_healthy  # waits for redis-cli ping to pass
    kafka:
      condition: service_healthy  # waits for kafka-broker-api-versions to pass
```

**Key snippet — overriding hosts for Docker networking:**
```yaml
app:
  env_file: .env              # loads your local values (e.g. DB_PASSWORD)
  environment:
    DB_HOST: postgres         # overrides localhost → Docker service name
    REDIS_ADDR: redis:6379    # overrides localhost → Docker service name
    KAFKA_BROKER: kafka:29092 # internal listener, not the host-exposed 9092
```

The `env_file` loads your `.env` for secrets, then the `environment` block overrides the host/port values to Docker service names — so the same `.env` file works for both local dev and Docker without changes.

---

## `Dockerfile` — Two-Stage Build

**What it does:** Builds the Go binary in a full Go environment, then copies only the compiled binary into a minimal Alpine image. The final image is ~15MB and runs as a non-root user.

**Key snippet — build stage:**
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download            # cached layer — only re-runs if go.mod changes
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \         # strip debug info — smaller binary
    -o /app/urlshortener \
    ./cmd/api
```

**Key snippet — runtime stage:**
```dockerfile
FROM alpine:3.19
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser                    # never run as root

COPY --from=builder /app/urlshortener .

HEALTHCHECK --interval=15s --timeout=5s --start-period=30s \
    CMD wget --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["./urlshortener"]
```

The two-stage build means the Go compiler, source code, and intermediate files never end up in the production image.

---

## How a Request Flows Through the Code

Here is the complete path of a `GET /abc1234` redirect request:

```
1. docker-compose.yml     → routes port 8080 to the app container
2. cmd/api/main.go        → http.Server receives the request
3. router.go              → chi matches /{shortCode}, runs middleware chain
4. middleware/logger.go   → records start time
5. middleware/recovery.go → sets up panic safety net
6. middleware/ratelimit.go → checks IP hasn't exceeded 60 req/min
7. url_handler.go         → Redirect() extracts "abc1234" from URL
8. url_service.go         → Resolve() checks Redis first
9. repository/redis.go    → Get() looks up "url:abc1234"
   ├─ CACHE HIT  → returns URL struct immediately
   └─ CACHE MISS → falls through to step 10
10. repository/postgres.go → GetByShortCode() runs SELECT WHERE short_code=$1
11. repository/redis.go    → Set() stores result for future requests
12. url_handler.go        → http.Redirect(302) sent to client ← fast path ends here
13. goroutine spawned     → PublishClickEvent() sends JSON to Kafka
14. kafka/consumer.go     → reads message from url-clicks topic (async)
15. repository/postgres.go → IncrementClickCount() runs UPDATE SET click_count+1
```

Steps 1–12 are the hot path — typically under 5ms. Steps 13–15 happen asynchronously and do not affect response time.
