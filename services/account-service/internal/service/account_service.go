package service

import (
	"context"
	"errors"
	"fmt"

	"avikmukherjee/m/account-service/internal/model"
	"avikmukherjee/m/account-service/internal/repository"
)

type AccountService struct {
	repo *repository.AccountRepository
}

func NewAccountService(repo *repository.AccountRepository) *AccountService {
	return &AccountService{repo: repo}
}

func (s *AccountService) CreateAccount(ctx context.Context, userID string, req *model.CreateAccountRequest) (*model.Account, error) {
	if req.AccountType != model.AccountTypeSavings && req.AccountType != model.AccountTypeCurrent {
		return nil, errors.New("account_type must be 'savings' or 'current'")
	}
	if req.Currency == "" {
		req.Currency = "INR"
	}

	return s.repo.Create(ctx, &model.Account{
		UserID:      userID,
		AccountType: req.AccountType,
		Currency:    req.Currency,
	})
}

func (s *AccountService) GetAccount(ctx context.Context, accountID, userID string) (*model.Account, error) {
	account, err := s.repo.FindByID(ctx, accountID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	// Ownership check — users can only see their own accounts
	if account.UserID != userID {
		return nil, repository.ErrNotFound
	}
	return account, nil
}

func (s *AccountService) ListAccounts(ctx context.Context, userID string) ([]*model.Account, error) {
	return s.repo.FindByUserID(ctx, userID)
}

// UpdateBalance is called by the transaction-service via internal API.
func (s *AccountService) UpdateBalance(ctx context.Context, req *model.BalanceUpdateRequest) (*model.Account, error) {
	if req.IdempotencyKey == "" {
		return nil, errors.New("idempotency_key is required")
	}
	if req.AccountID == "" {
		return nil, errors.New("account_id is required")
	}

	acc, err := s.repo.UpdateBalance(ctx, req)
	if errors.Is(err, repository.ErrInsufficientFunds) {
		return nil, fmt.Errorf("insufficient funds: %w", err)
	}
	if errors.Is(err, repository.ErrIdempotentReplay) {
		// Not an error — safe to return the current account state
		return s.repo.FindByID(ctx, req.AccountID)
	}
	return acc, err
}
