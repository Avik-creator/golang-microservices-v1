package model

import "time"

type AccountType string

const (
	AccountTypeSavings  AccountType = "savings"
	AccountTypeCurrent  AccountType = "current"
)

type Account struct {
	ID          string      `json:"id"`
	UserID      string      `json:"user_id"`
	AccountType AccountType `json:"account_type"`
	Balance     float64     `json:"balance"`   // stored as NUMERIC in DB
	Currency    string      `json:"currency"`  // e.g. "INR", "USD"
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type CreateAccountRequest struct {
	AccountType AccountType `json:"account_type"` // "savings" | "current"
	Currency    string      `json:"currency"`
}

// BalanceUpdateRequest is used internally by transaction-service via a
// service-to-service call (not exposed to the public API gateway).
type BalanceUpdateRequest struct {
	AccountID     string  `json:"account_id"`
	Amount        float64 `json:"amount"`         // positive = credit, negative = debit
	IdempotencyKey string `json:"idempotency_key"` // prevents double-processing
}
