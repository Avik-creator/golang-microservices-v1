package repository

import (
	"context"
	"errors"
	"fmt"

	"avikmukherjee/m/account-service/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("account not found")
var ErrInsufficientFunds = errors.New("insufficient funds")
var ErrIdempotentReplay = errors.New("idempotency key already processed")

type AccountRepository struct {
	db *pgxpool.Pool
}

func NewAccountRepository(db *pgxpool.Pool) *AccountRepository {
	return &AccountRepository{db: db}
}

func (r *AccountRepository) Migrate(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS accounts (
			id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id      UUID NOT NULL,
			account_type TEXT NOT NULL CHECK (account_type IN ('savings','current')),
			balance      NUMERIC(18,2) NOT NULL DEFAULT 0.00,
			currency     TEXT NOT NULL DEFAULT 'INR',
			created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS balance_updates (
			idempotency_key TEXT PRIMARY KEY,
			account_id      UUID NOT NULL,
			amount          NUMERIC(18,2) NOT NULL,
			processed_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_accounts_user_id ON accounts(user_id);
	`)
	return err
}

func (r *AccountRepository) Create(ctx context.Context, a *model.Account) (*model.Account, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO accounts (user_id, account_type, currency)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, account_type, balance, currency, created_at, updated_at
	`, a.UserID, a.AccountType, a.Currency)

	created := &model.Account{}
	err := row.Scan(
		&created.ID, &created.UserID, &created.AccountType,
		&created.Balance, &created.Currency, &created.CreatedAt, &created.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create account: %w", err)
	}
	return created, nil
}

func (r *AccountRepository) FindByID(ctx context.Context, id string) (*model.Account, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, user_id, account_type, balance, currency, created_at, updated_at
		FROM accounts WHERE id = $1
	`, id)
	return scanAccount(row)
}

func (r *AccountRepository) FindByUserID(ctx context.Context, userID string) ([]*model.Account, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, account_type, balance, currency, created_at, updated_at
		FROM accounts WHERE user_id = $1 ORDER BY created_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*model.Account
	for rows.Next() {
		a := &model.Account{}
		if err := rows.Scan(
			&a.ID, &a.UserID, &a.AccountType,
			&a.Balance, &a.Currency, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

// UpdateBalance atomically applies a debit or credit with idempotency protection.
// Uses a DB transaction to ensure the balance update and idempotency record are atomic.
func (r *AccountRepository) UpdateBalance(ctx context.Context, req *model.BalanceUpdateRequest) (*model.Account, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Check idempotency key
	var exists bool
	err = tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM balance_updates WHERE idempotency_key = $1)`,
		req.IdempotencyKey,
	).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrIdempotentReplay
	}

	// Lock the row and check balance for debits
	var currentBalance float64
	err = tx.QueryRow(ctx,
		`SELECT balance FROM accounts WHERE id = $1 FOR UPDATE`,
		req.AccountID,
	).Scan(&currentBalance)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if req.Amount < 0 && currentBalance+req.Amount < 0 {
		return nil, ErrInsufficientFunds
	}

	// Apply the balance change
	row := tx.QueryRow(ctx, `
		UPDATE accounts
		SET balance = balance + $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id, user_id, account_type, balance, currency, created_at, updated_at
	`, req.Amount, req.AccountID)

	updated := &model.Account{}
	if err := row.Scan(
		&updated.ID, &updated.UserID, &updated.AccountType,
		&updated.Balance, &updated.Currency, &updated.CreatedAt, &updated.UpdatedAt,
	); err != nil {
		return nil, err
	}

	// Record idempotency key
	_, err = tx.Exec(ctx,
		`INSERT INTO balance_updates (idempotency_key, account_id, amount) VALUES ($1, $2, $3)`,
		req.IdempotencyKey, req.AccountID, req.Amount,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return updated, nil
}

func scanAccount(row pgx.Row) (*model.Account, error) {
	a := &model.Account{}
	err := row.Scan(
		&a.ID, &a.UserID, &a.AccountType,
		&a.Balance, &a.Currency, &a.CreatedAt, &a.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan account: %w", err)
	}
	return a, nil
}
