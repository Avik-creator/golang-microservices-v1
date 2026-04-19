package kafka

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"avikmukherjee.com/m/transaction-service/internal/model"
	kafkago "github.com/segmentio/kafka-go"
)

const (
	TopicTransactionEvents = "transactions.events"
)

type Producer struct {
	writer *kafkago.Writer
}

func NewProducer(broker string) *Producer {
	writer := &kafkago.Writer{
		Addr:         kafkago.TCP(broker),
		Balancer:     &kafkago.LeastBytes{},
		MaxAttempts:  3,
		WriteTimeout: 10 * time.Second,
		// RequiredAcks: all replicas must ack (safe for fintech)
		RequiredAcks: kafkago.RequireAll,
	}
	log.Printf("[kafka-producer] ready, broker=%s", broker)
	return &Producer{writer: writer}
}

func (p *Producer) PublishTransactionEvent(ctx context.Context, event *model.TransactionEvent) error {
	// Use transaction ID as the Kafka message key so all events for the
	// same transaction land on the same partition (ordering guarantee).
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := kafkago.Message{
		Topic: TopicTransactionEvents,
		Key:   []byte(event.TransactionID),
		Value: payload,
		Headers: []kafkago.Header{
			{Key: "event_type", Value: []byte(event.EventType)},
		},
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		log.Printf("[kafka-producer] failed to publish event %s: %v", event.EventType, err)
		return err
	}

	log.Printf("[kafka-producer] published %s for tx=%s", event.EventType, event.TransactionID)
	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
