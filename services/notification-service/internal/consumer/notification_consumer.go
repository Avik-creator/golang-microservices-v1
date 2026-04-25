package consumer

import (
	"avikmukherjee/m/notification-service/internal/model"
	"avikmukherjee/m/notification-service/internal/service"
	"context"
	"encoding/json"
	"log"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

const (
	TopicTransactionEvents = "transactions.events"
	TopicFraudAlerts       = "fraud.alerts"
)

type NotificationConsumer struct {
	txReader    *kafkago.Reader
	fraudReader *kafkago.Reader
	mailer      *service.Mailer
}

func NewNotificationConsumer(broker, groupID string, mailer *service.Mailer) *NotificationConsumer {
	makeReader := func(topic string) *kafkago.Reader {
		return kafkago.NewReader(kafkago.ReaderConfig{
			Brokers:        []string{broker},
			Topic:          topic,
			GroupID:        groupID + "-" + topic, // separate group per topic
			MinBytes:       1,
			MaxBytes:       10e6,
			CommitInterval: time.Second,
			StartOffset:    kafkago.FirstOffset,
		})
	}

	return &NotificationConsumer{
		txReader:    makeReader(TopicTransactionEvents),
		fraudReader: makeReader(TopicFraudAlerts),
		mailer:      mailer,
	}
}

// Start launches two goroutines — one per topic — and blocks until ctx is done.
func (c *NotificationConsumer) Start(ctx context.Context) {
	log.Println("[notification-consumer] started, listening on topics:", TopicTransactionEvents, "&", TopicFraudAlerts)

	go c.consumeTransactions(ctx)
	go c.consumeFraudAlerts(ctx)

	<-ctx.Done()
	log.Println("[notification-consumer] context cancelled, stopping")
}

func (c *NotificationConsumer) consumeTransactions(ctx context.Context) {
	for {
		msg, err := c.txReader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[notification-consumer] tx fetch error: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		var event model.TransactionEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf("[notification-consumer] bad tx payload: %v", err)
			c.txReader.CommitMessages(ctx, msg)
			continue
		}

		// Only notify on terminal states
		if event.EventType != "transaction.completed" && event.EventType != "transaction.failed" {
			c.txReader.CommitMessages(ctx, msg)
			continue
		}

		email := service.BuildTransactionEmail(&event)
		if err := c.mailer.Send(email); err != nil {
			log.Printf("[notification-consumer] email send failed: %v", err)
			// Don't commit — will retry on restart
			continue
		}

		c.txReader.CommitMessages(ctx, msg)
	}
}

func (c *NotificationConsumer) consumeFraudAlerts(ctx context.Context) {
	for {
		msg, err := c.fraudReader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[notification-consumer] fraud fetch error: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		var alert model.FraudAlert
		if err := json.Unmarshal(msg.Value, &alert); err != nil {
			log.Printf("[notification-consumer] bad fraud payload: %v", err)
			c.fraudReader.CommitMessages(ctx, msg)
			continue
		}

		email := service.BuildFraudAlertEmail(&alert)
		if err := c.mailer.Send(email); err != nil {
			log.Printf("[notification-consumer] fraud alert email failed: %v", err)
			continue
		}

		c.fraudReader.CommitMessages(ctx, msg)
	}
}

func (c *NotificationConsumer) Close() {
	c.txReader.Close()
	c.fraudReader.Close()
}
