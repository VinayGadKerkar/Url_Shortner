# ─── Stage 1: Build ───────────────────────────────────────────────────────────
# Use the official Go image as a build environment.
# We build a statically linked binary so the final image needs no Go runtime.
FROM golang:1.21-alpine AS builder

# Install git (required for go module download) and ca-certificates.
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy dependency files first to leverage Docker layer cache.
# go.sum is downloaded separately so source changes don't re-download modules.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source.
COPY . .

# Build the binary.
# CGO_ENABLED=0 produces a statically linked binary compatible with scratch/alpine.
# -ldflags="-s -w" strips debug info to reduce binary size.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /app/urlshortener \
    ./cmd/api

# ─── Stage 2: Runtime ─────────────────────────────────────────────────────────
# Use a minimal alpine image — no Go toolchain, no shell in production.
FROM alpine:3.19

# ca-certificates needed for outbound HTTPS (e.g., custom long URLs with TLS).
RUN apk add --no-cache ca-certificates tzdata wget

# Create a non-root user for security.
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy the compiled binary from the builder stage.
COPY --from=builder /app/urlshortener .

# Migrations are embedded in the binary via go:embed, so no extra copy needed.

USER appuser

EXPOSE 8080

# Healthcheck: the /health endpoint must return 200 for Docker to consider
# the container healthy.
HEALTHCHECK --interval=15s --timeout=5s --start-period=30s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["./urlshortener"]
