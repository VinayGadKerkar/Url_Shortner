package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"urlshortener/config"
	"urlshortener/internal/handler"
	"urlshortener/internal/middleware"
	"urlshortener/internal/repository"
	"urlshortener/internal/service"
	kafkapkg "urlshortener/kafka"
	"urlshortener/migrations"
)

func main() {
	// Load .env if present (ignored in production where real env vars are set).
	_ = godotenv.Load()

	// --- Logger ---
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync() //nolint:errcheck

	// --- Config ---
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	// --- PostgreSQL ---
	pool, err := connectPostgres(cfg, logger)
	if err != nil {
		logger.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer pool.Close()

	// --- Run migrations ---
	if err := migrations.Run(context.Background(), pool, logger); err != nil {
		logger.Fatal("failed to run migrations", zap.Error(err))
	}

	// --- Redis ---
	redisClient := connectRedis(cfg, logger)
	defer redisClient.Close()

	// --- Repositories ---
	urlRepo := repository.NewPostgresURLRepository(pool)
	urlCache := repository.NewRedisURLCache(redisClient, cfg.Redis.CacheTTL)

	// --- Kafka producer ---
	producer := kafkapkg.NewProducer(cfg.Kafka.Brokers, cfg.Kafka.Topic, logger)
	defer producer.Close()

	// --- Kafka analytics consumer (runs in background) ---
	consumer := kafkapkg.NewConsumer(
		cfg.Kafka.Brokers,
		cfg.Kafka.Topic,
		"analytics-consumer-group",
		urlRepo,
		logger,
	)

	// Context that is cancelled on OS signal — used for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go consumer.Start(ctx)
	defer consumer.Close()

	// --- Services ---
	urlSvc := service.NewURLService(urlRepo, urlCache, logger, cfg.Server.BaseURL)

	// --- Handlers ---
	urlHandler := handler.NewURLHandler(urlSvc, producer, logger)
	healthHandler := handler.NewHealthHandler(urlCache, func(ctx context.Context) error {
		return pool.Ping(ctx)
	}, logger)

	// --- Router ---
	_ = middleware.RequestLogger // imported via router
	router := handler.NewRouter(handler.RouterDeps{
		URLHandler:        urlHandler,
		HealthHandler:     healthHandler,
		Logger:            logger,
		RequestsPerMinute: 60,
	})

	// --- HTTP server ---
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine so we can listen for shutdown signals.
	go func() {
		logger.Info("server starting", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	// Block until interrupt signal.
	<-ctx.Done()
	stop()

	// --- Graceful shutdown ---
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	logger.Info("shutting down server...")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}
	logger.Info("server stopped")
}

// connectPostgres creates a pgxpool with retry logic.
func connectPostgres(cfg *config.Config, logger *zap.Logger) (*pgxpool.Pool, error) {
	const maxRetries = 10
	const retryDelay = 3 * time.Second

	poolCfg, err := pgxpool.ParseConfig(cfg.Database.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}
	poolCfg.MaxConns = 20
	poolCfg.MinConns = 2

	for i := 1; i <= maxRetries; i++ {
		pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
		if err == nil {
			if pingErr := pool.Ping(context.Background()); pingErr == nil {
				logger.Info("connected to postgres")
				return pool, nil
			}
			pool.Close()
		}
		logger.Warn("waiting for postgres", zap.Int("attempt", i), zap.Error(err))
		time.Sleep(retryDelay)
	}
	return nil, fmt.Errorf("could not connect to postgres after %d attempts", maxRetries)
}

// connectRedis creates a Redis client.
func connectRedis(cfg *config.Config, logger *zap.Logger) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	const maxRetries = 10
	for i := 1; i <= maxRetries; i++ {
		if err := client.Ping(context.Background()).Err(); err == nil {
			logger.Info("connected to redis")
			return client
		}
		logger.Warn("waiting for redis", zap.Int("attempt", i))
		time.Sleep(2 * time.Second)
	}
	logger.Warn("redis unavailable, continuing without cache")
	return client
}
