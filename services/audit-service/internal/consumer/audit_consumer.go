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
	TopicDeadLetter        = "audit.dead-letter"
	maxRetries             = 3
)

type AuditConsumer struct {
	txReader    *kafkago.Reader
	fraudReader *kafkago.Reader
	dlq         *kafkago.Writer
	store       *service.AuditStore
	retries     map[string]int // key: msg key string, tracks per-message attempts
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

	dlq := &kafkago.Writer{
		Addr:         kafkago.TCP(broker),
		Topic:        TopicDeadLetter,
		Balancer:     &kafkago.LeastBytes{},
		MaxAttempts:  3,
		WriteTimeout: 10 * time.Second,
		RequiredAcks: kafkago.RequireAll,
	}

	return &AuditConsumer{
		txReader:    makeReader(TopicTransactionEvents),
		fraudReader: makeReader(TopicFraudAlerts),
		dlq:         dlq,
		store:       store,
		retries:     make(map[string]int),
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

		msgKey := fmt.Sprintf("%s:%d:%d", topic, msg.Partition, msg.Offset)

		if err := c.record(ctx, msg, topic, eventType); err != nil {
			c.retries[msgKey]++
			log.Printf("[audit-consumer] record error (attempt %d/%d) offset=%d: %v",
				c.retries[msgKey], maxRetries, msg.Offset, err)

			if c.retries[msgKey] >= maxRetries {
				log.Printf("[audit-consumer] max retries reached, routing offset=%d to DLQ", msg.Offset)
				c.sendToDLQ(ctx, msg, topic, err)
				delete(c.retries, msgKey)
				reader.CommitMessages(ctx, msg) //nolint:errcheck
			} else {
				time.Sleep(time.Second)
			}
			continue
		}

		delete(c.retries, msgKey)
		if err := reader.CommitMessages(ctx, msg); err != nil {
			log.Printf("[audit-consumer] commit error: %v", err)
		}
	}
}

// record deserialises the raw Kafka message and writes an AuditRecord to MinIO.
func (c *AuditConsumer) record(
	ctx context.Context,
	msg kafkago.Message,
	topic string,
	eventType model.AuditEventType,
) error {
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

func (c *AuditConsumer) sendToDLQ(ctx context.Context, msg kafkago.Message, sourceTopic string, cause error) {
	if err := c.dlq.WriteMessages(ctx, kafkago.Message{
		Key:   msg.Key,
		Value: msg.Value,
		Headers: []kafkago.Header{
			{Key: "source-topic", Value: []byte(sourceTopic)},
			{Key: "error", Value: []byte(cause.Error())},
		},
	}); err != nil {
		log.Printf("[audit-consumer] DLQ write failed: %v", err)
	}
}

func (c *AuditConsumer) Close() {
	c.txReader.Close()
	c.fraudReader.Close()
	c.dlq.Close()
}

func generateID() string {
	return uuid.New().String()
}
