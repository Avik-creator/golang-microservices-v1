package model

import "time"

type TransactionStatus string
type TransactionType string

const (
	StatusPending   TransactionStatus = "pending"
	StatusCompleted TransactionStatus = "completed"
	StatusFailed    TransactionStatus = "failed"

	TypeDebit  TransactionType = "debit"
	TypeCredit TransactionType = "credit"
)

type Transaction struct {
	ID             string            `json:"id"`
	FromAccountID  string            `json:"from_account_id"`
	ToAccountID    string            `json:"to_account_id"`
	Amount         float64           `json:"amount"`
	Currency       string            `json:"currency"`
	Status         TransactionStatus `json:"status"`
	Description    string            `json:"description"`
	IdempotencyKey string            `json:"idempotency_key"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type CreateTransactionRequest struct {
	FromAccountID  string  `json:"from_account_id"`
	ToAccountID    string  `json:"to_account_id"`
	Amount         float64 `json:"amount"`
	Currency       string  `json:"currency"`
	Description    string  `json:"description"`
	IdempotencyKey string  `json:"idempotency_key"` // client-supplied for dedup
}

// TransactionEvent is the Kafka message payload published after a transaction.
type TransactionEvent struct {
	TransactionID string            `json:"transaction_id"`
	FromAccountID string            `json:"from_account_id"`
	ToAccountID   string            `json:"to_account_id"`
	Amount        float64           `json:"amount"`
	Currency      string            `json:"currency"`
	Status        TransactionStatus `json:"status"`
	EventType     string            `json:"event_type"` // "transaction.completed" | "transaction.failed"
	OccurredAt    time.Time         `json:"occurred_at"`
}
