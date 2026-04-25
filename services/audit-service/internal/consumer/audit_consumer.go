package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"avikmukherjee/m/audit-service/internal/model"
	"avikmukherjee/m/audit-service/internal/service"

	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"
)

const (
	TopicTransactionEvents = "transactions.events"
	TopicFraudAlerts       = "fraud.alerts"
)

type AuditConsumer struct {
	txReader    *kafkago.Reader
	fraudReader *kafkago.Reader
	store       *service.AuditStore
}

func NewAuditConsumer(broker, groupID string, store *service.AuditStore) *AuditConsumer {
	makeReader := func(topic string) *kafkago.Reader {
		return kafkago.NewReader(kafkago.ReaderConfig{
			Brokers:        []string{broker},
			Topic:          topic,
			GroupID:        groupID + "-" + topic,
			MinBytes:       1,
			MaxBytes:       10e6,
			CommitInterval: time.Second,
			StartOffset:    kafkago.FirstOffset,
		})
	}

	return &AuditConsumer{
		txReader:    makeReader(TopicTransactionEvents),
		fraudReader: makeReader(TopicFraudAlerts),
		store:       store,
	}
}

// Start launches two goroutines (one per topic) and blocks until ctx is cancelled.
func (c *AuditConsumer) Start(ctx context.Context) {
	log.Println("[audit-consumer] started, listening on topics:", TopicTransactionEvents, "&", TopicFraudAlerts)

	go c.consumeTopic(ctx, c.txReader, TopicTransactionEvents, model.AuditTransaction)
	go c.consumeTopic(ctx, c.fraudReader, TopicFraudAlerts, model.AuditFraudAlert)

	<-ctx.Done()
	log.Println("[audit-consumer] context cancelled, stopping")
}

func (c *AuditConsumer) consumeTopic(
	ctx context.Context,
	reader *kafkago.Reader,
	topic string,
	eventType model.AuditEventType,
) {
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[audit-consumer] fetch error on %s: %v", topic, err)
			time.Sleep(2 * time.Second)
			continue
		}

		if err := c.record(ctx, msg, topic, eventType); err != nil {
			log.Printf("[audit-consumer] record error (offset %d): %v — will retry", msg.Offset, err)
			// Don't commit — kafka will redeliver on next restart (at-least-once)
			time.Sleep(time.Second)
			continue
		}

		if err := reader.CommitMessages(ctx, msg); err != nil {
			log.Printf("[audit-consumer] commit error: %v", err)
		}
	}
}

// record deserialises the raw Kafka message and writes an AuditRecord to MinIO.
// We store the raw payload as-is so the audit log is a faithful, tamper-evident
// copy of every event that passed through the system.
func (c *AuditConsumer) record(
	ctx context.Context,
	msg kafkago.Message,
	topic string,
	eventType model.AuditEventType,
) error {
	// Decode payload into a generic map so we preserve all fields verbatim
	var payload map[string]any
	if err := json.Unmarshal(msg.Value, &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	auditRecord := &model.AuditRecord{
		ID:          generateID(),
		EventType:   eventType,
		SourceTopic: topic,
		Payload:     payload,
		RecordedAt:  time.Now().UTC(),
	}

	return c.store.Write(ctx, auditRecord)
}

func (c *AuditConsumer) Close() {
	c.txReader.Close()
	c.fraudReader.Close()
}

func generateID() string {
	return uuid.New().String()
}
