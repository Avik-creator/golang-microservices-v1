package consumer

import (
	"avikmukherjee/m/notification-service/internal/service"
	"context"
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
