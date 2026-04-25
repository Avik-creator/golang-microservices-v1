package consumer

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"avikmukherjee/m/fraud-service/internal/model"
	"avikmukherjee/m/fraud-service/internal/service"

	kafkago "github.com/segmentio/kafka-go"
)

const (
	TopicTransactionEvents = "transactions.events"
	TopicFraudAlerts       = "fraud.alerts"
)

type FraudConsumer struct {
	reader *kafkago.Reader
	writer *kafkago.Writer
	engine *service.FraudEngine
}

func NewFraudConsumer(broker, groupID string, engine *service.FraudEngine) *FraudConsumer {
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        []string{broker},
		Topic:          TopicTransactionEvents,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: time.Second,
		// Start from the earliest unread offset for this consumer group
		StartOffset: kafkago.FirstOffset,
	})

	writer := &kafkago.Writer{
		Addr:         kafkago.TCP(broker),
		Balancer:     &kafkago.LeastBytes{},
		MaxAttempts:  3,
		WriteTimeout: 10 * time.Second,
		RequiredAcks: kafkago.RequireAll,
	}

	return &FraudConsumer{
		reader: reader,
		writer: writer,
		engine: engine,
	}
}

// Start begins consuming transaction events, scoring each one.
// Blocks until ctx is cancelled (called in a goroutine from main).
func (c *FraudConsumer) Start(ctx context.Context) {
	log.Println("[fraud-consumer] started, listening on topic:", TopicTransactionEvents)

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				// Context cancelled — clean shutdown
				log.Println("[fraud-consumer] stopping")
				return
			}
			log.Printf("[fraud-consumer] fetch error: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if err := c.process(ctx, msg); err != nil {
			log.Printf("[fraud-consumer] process error for offset %d: %v", msg.Offset, err)
			// Do NOT commit — message will be redelivered on restart.
			// In production, route to a dead-letter topic after N retries.
			continue
		}

		// Commit only after successful processing (at-least-once delivery)
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			log.Printf("[fraud-consumer] commit error: %v", err)
		}
	}
}

func (c *FraudConsumer) process(ctx context.Context, msg kafkago.Message) error {
	var event model.TransactionEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		log.Printf("[fraud-consumer] bad message payload: %v", err)
		return nil // malformed — skip, don't retry
	}

	// Only evaluate completed transactions
	if event.EventType != "transaction.completed" {
		return nil
	}

	result := c.engine.Evaluate(&event)

	log.Printf("[fraud-consumer] tx=%s score=%d flagged=%v reasons=%v",
		result.TransactionID, result.Score, result.Flagged, result.Reasons)

	if result.Flagged {
		return c.publishAlert(ctx, &event, result)
	}
	return nil
}

func (c *FraudConsumer) publishAlert(ctx context.Context, event *model.TransactionEvent, result *model.FraudResult) error {
	alert := model.FraudAlert{
		TransactionID: event.TransactionID,
		FromAccountID: event.FromAccountID,
		Amount:        event.Amount,
		Currency:      event.Currency,
		Score:         result.Score,
		Reasons:       result.Reasons,
		AlertedAt:     time.Now(),
	}

	payload, err := json.Marshal(alert)
	if err != nil {
		return err
	}

	err = c.writer.WriteMessages(ctx, kafkago.Message{
		Topic: TopicFraudAlerts,
		Key:   []byte(alert.TransactionID),
		Value: payload,
	})
	if err != nil {
		return err
	}

	log.Printf("[fraud-consumer] 🚨 fraud alert published for tx=%s score=%d", alert.TransactionID, alert.Score)
	return nil
}

func (c *FraudConsumer) Close() {
	c.reader.Close()
	c.writer.Close()
}
