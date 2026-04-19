package repository

import (
	"context"
	"errors"
	"fmt"
	"avikmukherjee.com/m/transaction-service/internal/model"
	"github.com/jackc/pgx/v5"
"github.com/jackc/pgx/v5/pgxpool"
)


var ErrNotFound = errors.New("Transaction not found")
var ErrDuplicateKey = errors.New("Idempotency Duplicate key")

type TransactionRepository struct {
	db *pgxpool.Pool
}

func NewTransactionRepository(db *pgxpool.Pool) *TransactionRepository {
	return &TransactionRepository{db: db}
}

func (r *TransactionRepository) Migrate(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS transactions (
			id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			from_account_id UUID NOT NULL,
			to_account_id   UUID NOT NULL,
			amount          NUMERIC(18,2) NOT NULL,
			currency        TEXT NOT NULL DEFAULT 'INR',
			status          TEXT NOT NULL DEFAULT 'pending',
			description     TEXT,
			idempotency_key TEXT UNIQUE NOT NULL,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_tx_from_account ON transactions(from_account_id);
		CREATE INDEX IF NOT EXISTS idx_tx_to_account   ON transactions(to_account_id);
	`)
	return err
}
func (r *TransactionRepository) Create(ctx context.Context, t *model.Transaction) (*model.Transaction, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO transactions
			(from_account_id, to_account_id, amount, currency, status, description, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, from_account_id, to_account_id, amount, currency, status, description, idempotency_key, created_at, updated_at
	`, t.FromAccountID, t.ToAccountID, t.Amount, t.Currency,
		model.StatusPending, t.Description, t.IdempotencyKey)

	created := &model.Transaction{}
	err := scanTransaction(row, created)
	if err != nil {
		if err.Error() == `ERROR: duplicate key value violates unique constraint "transactions_idempotency_key_key" (SQLSTATE 23505)` {
			return nil, ErrDuplicateKey
		}
		return nil, fmt.Errorf("create transaction: %w", err)
	}
	return created, nil
}

func (r *TransactionRepository) UpdateStatus(ctx context.Context, id string, status model.TransactionStatus) (*model.Transaction, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE transactions
		SET status = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id, from_account_id, to_account_id, amount, currency, status, description, idempotency_key, created_at, updated_at
	`, status, id)

	t := &model.Transaction{}
	if err := scanTransaction(row, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (r *TransactionRepository) FindByID(ctx context.Context, id string) (*model.Transaction, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, from_account_id, to_account_id, amount, currency, status, description, idempotency_key, created_at, updated_at
		FROM transactions WHERE id = $1
	`, id)
	t := &model.Transaction{}
	if err := scanTransaction(row, t); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return t, nil
}

func (r *TransactionRepository) FindByAccountID(ctx context.Context, accountID string) ([]*model.Transaction, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, from_account_id, to_account_id, amount, currency, status, description, idempotency_key, created_at, updated_at
		FROM transactions
		WHERE from_account_id = $1 OR to_account_id = $1
		ORDER BY created_at DESC
		LIMIT 50
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txs []*model.Transaction
	for rows.Next() {
		t := &model.Transaction{}
		if err := rows.Scan(
			&t.ID, &t.FromAccountID, &t.ToAccountID,
			&t.Amount, &t.Currency, &t.Status,
			&t.Description, &t.IdempotencyKey,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, err
		}
		txs = append(txs, t)
	}
	return txs, rows.Err()
}

func scanTransaction(row pgx.Row, t *model.Transaction) error {
	return row.Scan(
		&t.ID, &t.FromAccountID, &t.ToAccountID,
		&t.Amount, &t.Currency, &t.Status,
		&t.Description, &t.IdempotencyKey,
		&t.CreatedAt, &t.UpdatedAt,
	)
}
