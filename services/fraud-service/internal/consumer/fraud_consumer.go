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
	TopicDeadLetter        = "fraud.dead-letter"
	maxRetries             = 3
)

type FraudConsumer struct {
	reader  *kafkago.Reader
	writer  *kafkago.Writer
	dlq     *kafkago.Writer
	engine  *service.FraudEngine
	retries map[string]int // key: partition:offset string
}

func NewFraudConsumer(broker, groupID string, engine *service.FraudEngine) *FraudConsumer {
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        []string{broker},
		Topic:          TopicTransactionEvents,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: time.Second,
		StartOffset:    kafkago.FirstOffset,
	})

	makeWriter := func(topic string) *kafkago.Writer {
		return &kafkago.Writer{
			Addr:         kafkago.TCP(broker),
			Topic:        topic,
			Balancer:     &kafkago.LeastBytes{},
			MaxAttempts:  3,
			WriteTimeout: 10 * time.Second,
			RequiredAcks: kafkago.RequireAll,
		}
	}

	return &FraudConsumer{
		reader:  reader,
		writer:  makeWriter(TopicFraudAlerts),
		dlq:     makeWriter(TopicDeadLetter),
		engine:  engine,
		retries: make(map[string]int),
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
				log.Println("[fraud-consumer] stopping")
				return
			}
			log.Printf("[fraud-consumer] fetch error: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		msgKey := msgID(msg)

		if err := c.process(ctx, msg); err != nil {
			c.retries[msgKey]++
			log.Printf("[fraud-consumer] process error (attempt %d/%d) offset=%d: %v",
				c.retries[msgKey], maxRetries, msg.Offset, err)

			if c.retries[msgKey] >= maxRetries {
				log.Printf("[fraud-consumer] max retries reached, routing offset=%d to DLQ", msg.Offset)
				c.sendToDLQ(ctx, msg, err)
				delete(c.retries, msgKey)
				c.reader.CommitMessages(ctx, msg) //nolint:errcheck
			}
			continue
		}

		delete(c.retries, msgKey)
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			log.Printf("[fraud-consumer] commit error: %v", err)
		}
	}
}

func (c *FraudConsumer) process(ctx context.Context, msg kafkago.Message) error {
	var event model.TransactionEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		log.Printf("[fraud-consumer] bad message payload: %v", err)
		return nil // malformed — skip immediately, no retry
	}

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

	if err := c.writer.WriteMessages(ctx, kafkago.Message{
		Key:   []byte(alert.TransactionID),
		Value: payload,
	}); err != nil {
		return err
	}

	log.Printf("[fraud-consumer] fraud alert published for tx=%s score=%d", alert.TransactionID, alert.Score)
	return nil
}

func (c *FraudConsumer) sendToDLQ(ctx context.Context, msg kafkago.Message, cause error) {
	if err := c.dlq.WriteMessages(ctx, kafkago.Message{
		Key:   msg.Key,
		Value: msg.Value,
		Headers: []kafkago.Header{
			{Key: "source-topic", Value: []byte(TopicTransactionEvents)},
			{Key: "error", Value: []byte(cause.Error())},
		},
	}); err != nil {
		log.Printf("[fraud-consumer] DLQ write failed: %v", err)
	}
}

func (c *FraudConsumer) Close() {
	c.reader.Close()
	c.writer.Close()
	c.dlq.Close()
}

func msgID(msg kafkago.Message) string {
	return string(msg.Key)
}
