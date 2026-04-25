package model

import "time"

// TransactionEvent consumed from transactions.events topic
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

// FraudAlert consumed from fraud.alerts topic
type FraudAlert struct {
	TransactionID string    `json:"transaction_id"`
	FromAccountID string    `json:"from_account_id"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	Score         int       `json:"score"`
	Reasons       []string  `json:"reasons"`
	AlertedAt     time.Time `json:"alerted_at"`
}

// Email is the internal representation of an outgoing email
type Email struct {
	To      string
	Subject string
	Body    string
}
