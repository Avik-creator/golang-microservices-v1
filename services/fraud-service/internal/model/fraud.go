package model

import "time"

// TransactionEvent mirrors what transaction-service publishes to Kafka.
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

type FraudReason string

const (
	ReasonLargeTransaction  FraudReason = "large_transaction"
	ReasonHighFrequency     FraudReason = "high_frequency"
	ReasonRoundAmount       FraudReason = "round_amount"
	ReasonSelfTransfer      FraudReason = "self_transfer"
)

// FraudResult is the output of the fraud engine for one transaction.
type FraudResult struct {
	TransactionID string        `json:"transaction_id"`
	Flagged       bool          `json:"flagged"`
	Score         int           `json:"score"`   // 0–100, higher = riskier
	Reasons       []FraudReason `json:"reasons"`
	EvaluatedAt   time.Time     `json:"evaluated_at"`
}

// FraudAlert is what gets published to Kafka when a transaction is flagged.
type FraudAlert struct {
	TransactionID string        `json:"transaction_id"`
	FromAccountID string        `json:"from_account_id"`
	Amount        float64       `json:"amount"`
	Currency      string        `json:"currency"`
	Score         int           `json:"score"`
	Reasons       []FraudReason `json:"reasons"`
	AlertedAt     time.Time     `json:"alerted_at"`
}
