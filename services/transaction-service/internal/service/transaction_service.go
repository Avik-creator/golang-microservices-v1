package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"avikmukherjee.com/m/transaction-service/internal/kafka"
	"avikmukherjee.com/m/transaction-service/internal/model"
	"avikmukherjee.com/m/transaction-service/internal/repository"
)

type TransactionService struct {
	repo              *repository.TransactionRepository
	producer          *kafka.Producer
	accountServiceURL string
	internalSecret    string
	httpClient        *http.Client
}

func NewTransactionService(
	repo *repository.TransactionRepository,
	producer *kafka.Producer,
	accountServiceURL string,
	internalSecret string,
) *TransactionService {
	return &TransactionService{
		repo:              repo,
		producer:          producer,
		accountServiceURL: accountServiceURL,
		internalSecret:    internalSecret,
		httpClient:        &http.Client{Timeout: 10 * time.Second},
	}
}

// ProcessPayment is the core saga:
//  1. Create transaction record (pending)
//  2. Debit sender via account-service
//  3. Credit receiver via account-service
//  4. Mark transaction completed
//  5. Publish Kafka event
//
// On any failure after step 1, we mark the transaction as failed and publish
// a failure event so downstream services can react.
func (s *TransactionService) ProcessPayment(ctx context.Context, req *model.CreateTransactionRequest) (*model.Transaction, error) {
	if req.Amount <= 0 {
		return nil, errors.New("amount must be greater than 0")
	}
	if req.FromAccountID == req.ToAccountID {
		return nil, errors.New("cannot transfer to the same account")
	}
	if req.IdempotencyKey == "" {
		return nil, errors.New("idempotency_key is required")
	}
	if req.Currency == "" {
		req.Currency = "INR"
	}

	// Step 1: Record the transaction as pending
	tx, err := s.repo.Create(ctx, &model.Transaction{
		FromAccountID:  req.FromAccountID,
		ToAccountID:    req.ToAccountID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Description:    req.Description,
		IdempotencyKey: req.IdempotencyKey,
	})
	if errors.Is(err, repository.ErrDuplicateKey) {
		return nil, errors.New("duplicate transaction: idempotency_key already used")
	}
	if err != nil {
		return nil, fmt.Errorf("create transaction record: %w", err)
	}

	// Steps 2 & 3: Debit sender, credit receiver
	if err := s.applyBalances(ctx, tx); err != nil {
		log.Printf("[transaction-service] balance update failed for tx=%s: %v", tx.ID, err)
		failed, _ := s.repo.UpdateStatus(ctx, tx.ID, model.StatusFailed)
		s.publishEvent(ctx, failed, "transaction.failed")
		return failed, fmt.Errorf("payment failed: %w", err)
	}

	// Step 4: Mark completed
	completed, err := s.repo.UpdateStatus(ctx, tx.ID, model.StatusCompleted)
	if err != nil {
		return tx, fmt.Errorf("status update failed: %w", err)
	}

	// Step 5: Publish success event
	s.publishEvent(ctx, completed, "transaction.completed")

	return completed, nil
}

func (s *TransactionService) GetTransaction(ctx context.Context, id string) (*model.Transaction, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *TransactionService) ListByAccount(ctx context.Context, accountID string) ([]*model.Transaction, error) {
	return s.repo.FindByAccountID(ctx, accountID)
}

// applyBalances calls account-service to debit sender and credit receiver.
func (s *TransactionService) applyBalances(ctx context.Context, tx *model.Transaction) error {
	// Debit sender
	if err := s.callBalanceUpdate(ctx, tx.FromAccountID, -tx.Amount, "debit-"+tx.ID); err != nil {
		return fmt.Errorf("debit failed: %w", err)
	}
	// Credit receiver
	if err := s.callBalanceUpdate(ctx, tx.ToAccountID, tx.Amount, "credit-"+tx.ID); err != nil {
		// Compensate: refund the sender (saga rollback)
		log.Printf("[transaction-service] credit failed, compensating debit for tx=%s", tx.ID)
		if refundErr := s.callBalanceUpdate(ctx, tx.FromAccountID, tx.Amount, "refund-"+tx.ID); refundErr != nil {
			log.Printf("[transaction-service] CRITICAL: refund failed for tx=%s: %v", tx.ID, refundErr)
		}
		return fmt.Errorf("credit failed: %w", err)
	}
	return nil
}

type balanceUpdatePayload struct {
	AccountID      string  `json:"account_id"`
	Amount         float64 `json:"amount"`
	IdempotencyKey string  `json:"idempotency_key"`
}

func (s *TransactionService) callBalanceUpdate(ctx context.Context, accountID string, amount float64, idempotencyKey string) error {
	payload, _ := json.Marshal(balanceUpdatePayload{
		AccountID:      accountID,
		Amount:         amount,
		IdempotencyKey: idempotencyKey,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.accountServiceURL+"/internal/accounts/balance",
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Secret", s.internalSecret)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("account-service unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("account-service error: %s", errResp["error"])
	}
	return nil
}

func (s *TransactionService) publishEvent(ctx context.Context, tx *model.Transaction, eventType string) {
	if tx == nil {
		return
	}
	event := &model.TransactionEvent{
		TransactionID: tx.ID,
		FromAccountID: tx.FromAccountID,
		ToAccountID:   tx.ToAccountID,
		Amount:        tx.Amount,
		Currency:      tx.Currency,
		Status:        tx.Status,
		EventType:     eventType,
		OccurredAt:    time.Now(),
	}
	// Fire-and-forget: don't block the HTTP response on Kafka publish
	go func() {
		if err := s.producer.PublishTransactionEvent(context.Background(), event); err != nil {
			log.Printf("[transaction-service] kafka publish failed: %v", err)
		}
	}()
}
