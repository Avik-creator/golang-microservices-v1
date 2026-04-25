package service

import (
	"sync"
	"time"

	"avikmukherjee/m/fraud-service/internal/model"
)

// FraudEngine evaluates transactions using rule-based scoring.
// Each rule contributes a score (0-100 total). A transaction is
// flagged when the combined score crosses 50.
type FraudEngine struct {
	largeTxThreshold float64
	maxTxPerMinute   int

	// In-memory sliding window: accountID → list of recent tx timestamps.
	// In production, replace with Redis for multi-instance support.
	mu      sync.Mutex
	txTimes map[string][]time.Time
}

func NewFraudEngine(largeTxThreshold float64, maxTxPerMinute int) *FraudEngine {
	return &FraudEngine{
		largeTxThreshold: largeTxThreshold,
		maxTxPerMinute:   maxTxPerMinute,
		txTimes:          make(map[string][]time.Time),
	}
}

// Evaluate scores a transaction event and returns a FraudResult.
func (e *FraudEngine) Evaluate(event *model.TransactionEvent) *model.FraudResult {
	result := &model.FraudResult{
		TransactionID: event.TransactionID,
		EvaluatedAt:   time.Now(),
	}

	// ── Rule 1: Large transaction ─────────────────────────────────────
	// Transactions above the configured threshold are high risk.
	if event.Amount >= e.largeTxThreshold {
		result.Score += 40
		result.Reasons = append(result.Reasons, model.ReasonLargeTransaction)
	}

	// ── Rule 2: High frequency ────────────────────────────────────────
	// More than N transactions from the same account within 1 minute.
	if e.isHighFrequency(event.FromAccountID, event.OccurredAt) {
		result.Score += 35
		result.Reasons = append(result.Reasons, model.ReasonHighFrequency)
	}

	// ── Rule 3: Suspiciously round amount ─────────────────────────────
	// e.g. exactly 50000, 100000 — common in structuring fraud.
	if isRoundAmount(event.Amount) {
		result.Score += 15
		result.Reasons = append(result.Reasons, model.ReasonRoundAmount)
	}

	// ── Rule 4: Self transfer ─────────────────────────────────────────
	// Sending to your own account — could be money laundering layering.
	if event.FromAccountID == event.ToAccountID {
		result.Score += 10
		result.Reasons = append(result.Reasons, model.ReasonSelfTransfer)
	}

	// Cap score at 100
	if result.Score > 100 {
		result.Score = 100
	}

	result.Flagged = result.Score >= 50

	return result
}

// isHighFrequency records this tx timestamp and checks if the account
// has exceeded the allowed rate within the last 60 seconds.
func (e *FraudEngine) isHighFrequency(accountID string, at time.Time) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	cutoff := at.Add(-1 * time.Minute)

	// Prune old entries
	existing := e.txTimes[accountID]
	fresh := existing[:0]
	for _, t := range existing {
		if t.After(cutoff) {
			fresh = append(fresh, t)
		}
	}
	fresh = append(fresh, at)
	e.txTimes[accountID] = fresh

	return len(fresh) > e.maxTxPerMinute
}

// isRoundAmount returns true for amounts that are exact multiples of 1000
// with no sub-unit component — a common pattern in structuring fraud.
func isRoundAmount(amount float64) bool {
	return amount >= 1000 && int(amount)%1000 == 0 && amount == float64(int(amount))
}
