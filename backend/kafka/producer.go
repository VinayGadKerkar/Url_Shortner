package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"urlshortener/internal/models"
)

// zapLogger adapts zap.Logger to kafka-go's logger interfaces.
type zapLogger struct{ logger *zap.Logger }

func (z *zapLogger) Printf(msg string, args ...interface{}) {
	z.logger.Debug(fmt.Sprintf(msg, args...))
}

// Producer wraps a kafka-go Writer and publishes ClickEvents to the topic.
type Producer struct {
	writer *kafkago.Writer
	logger *zap.Logger
}

// NewProducer creates a synchronous Kafka producer.
// Async=false means WriteMessages blocks until the broker acknowledges the
// write — this gives us real error signals. The caller (redirect handler)
// already runs this in a goroutine with a 10s timeout so redirect latency
// is unaffected.
func NewProducer(brokers []string, topic string, logger *zap.Logger) *Producer {
	writer := &kafkago.Writer{
		Addr:                   kafkago.TCP(brokers...),
		Topic:                  topic,
		Balancer:               &kafkago.LeastBytes{},
		BatchTimeout:           10 * time.Millisecond,
		Async:                  false, // synchronous — real ack, real errors
		RequiredAcks:           kafkago.RequireOne,
		AllowAutoTopicCreation: true, // create topic if it doesn't exist
		Logger:                 &zapLogger{logger: logger},
		ErrorLogger:            &zapLogger{logger: logger},
	}
	return &Producer{writer: writer, logger: logger}
}

// PublishClickEvent serialises a ClickEvent and sends it to Kafka.
// Returns a real error if the broker rejected the write.
func (p *Producer) PublishClickEvent(ctx context.Context, event models.ClickEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("kafka marshal click event: %w", err)
	}

	msg := kafkago.Message{
		Key:   []byte(event.ShortCode),
		Value: data,
		Time:  event.AccessedAt,
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		p.logger.Error("failed to publish click event to kafka",
			zap.Error(err),
			zap.String("short_code", event.ShortCode),
		)
		return err
	}

	p.logger.Info("click event published", zap.String("short_code", event.ShortCode))
	return nil
}

// Close gracefully shuts down the Kafka writer and flushes pending messages.
func (p *Producer) Close() error {
	return p.writer.Close()
}
