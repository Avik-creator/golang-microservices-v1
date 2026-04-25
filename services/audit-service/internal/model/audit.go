package model

import "time"

// AuditEventType classifies what kind of event is being logged.
type AuditEventType string

const (
	AuditTransaction AuditEventType = "transaction"
	AuditFraudAlert  AuditEventType = "fraud_alert"
)

// AuditRecord is the immutable document written to MinIO.
// Once written it is never modified — append-only by design.
type AuditRecord struct {
	ID          string         `json:"id"`           // UUID generated at audit time
	EventType   AuditEventType `json:"event_type"`
	SourceTopic string         `json:"source_topic"` // which Kafka topic this came from
	Payload     any            `json:"payload"`      // the original event, verbatim
	RecordedAt  time.Time      `json:"recorded_at"`
}

// StoragePath returns the MinIO object key for this record.
// Layout: <event_type>/<YYYY>/<MM>/<DD>/<id>.json
// This partitioning makes it easy to query logs by date range.
func (a *AuditRecord) StoragePath() string {
	t := a.RecordedAt.UTC()
	return string(a.EventType) + "/" +
		t.Format("2006") + "/" +
		t.Format("01") + "/" +
		t.Format("02") + "/" +
		a.ID + ".json"
}

// ── Inbound event shapes ─────────────────────────────────────────────

type TransactionEvent struct {
	TransactionID string    `json:"transaction_id"`
	FromAccountID string    `json:"from_account_id"`
	ToAccountID   string    `json:"to_account_id"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	Status        string    `json:"status"`
	EventType     string    `json:"event_type"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type FraudAlert struct {
	TransactionID string    `json:"transaction_id"`
	FromAccountID string    `json:"from_account_id"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	Score         int       `json:"score"`
	Reasons       []string  `json:"reasons"`
	AlertedAt     time.Time `json:"alerted_at"`
}
