package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"urlshortener/internal/models"
	"urlshortener/internal/repository"
)

// Consumer reads ClickEvents from Kafka and updates analytics in PostgreSQL.
// It runs as a long-lived goroutine started at application boot.
type Consumer struct {
	reader *kafkago.Reader
	repo   repository.URLRepository
	logger *zap.Logger
}

// NewConsumer creates a new analytics consumer.
func NewConsumer(brokers []string, topic, groupID string, repo repository.URLRepository, logger *zap.Logger) *Consumer {
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: time.Second,
		StartOffset:    kafkago.FirstOffset,
		Logger:         &zapLogger{logger: logger},
		ErrorLogger:    &zapLogger{logger: logger},
	})
	return &Consumer{reader: reader, repo: repo, logger: logger}
}

// Start begins the consume loop. It blocks until ctx is cancelled.
// Call this in a separate goroutine: go consumer.Start(ctx)
func (c *Consumer) Start(ctx context.Context) {
	c.logger.Info("analytics consumer started")

	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				// Context cancelled — clean shutdown.
				c.logger.Info("analytics consumer stopping")
				return
			}
			c.logger.Error("kafka read error", zap.Error(err))
			// Back-off before retrying to avoid tight error loops.
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}

		c.handleMessage(ctx, msg)
	}
}

// handleMessage deserialises a ClickEvent and increments the click counter.
func (c *Consumer) handleMessage(ctx context.Context, msg kafkago.Message) {
	var event models.ClickEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		c.logger.Error("failed to unmarshal click event",
			zap.Error(err),
			zap.ByteString("raw", msg.Value),
		)
		return
	}

	if err := c.repo.IncrementClickCount(ctx, event.ShortCode); err != nil {
		c.logger.Error("failed to increment click count",
			zap.Error(err),
			zap.String("short_code", event.ShortCode),
		)
		return
	}

	c.logger.Info("click count updated",
		zap.String("short_code", event.ShortCode),
		zap.Time("accessed_at", event.AccessedAt),
	)
}

// Close shuts down the Kafka reader.
func (c *Consumer) Close() error {
	if err := c.reader.Close(); err != nil {
		return fmt.Errorf("kafka consumer close: %w", err)
	}
	return nil
}
