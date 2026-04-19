package repository

import (
	"context"
	"errors"
	"fmt"

	"avikmukherjee.com/m/user-service/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("user not found")
var ErrDuplicate = errors.New("email already registered")

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// Migrate creates the users table if it doesn't exist.
func (r *UserRepository) Migrate(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email         TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			full_name     TEXT NOT NULL,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func (r *UserRepository) Create(ctx context.Context, u *model.User) (*model.User, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, full_name)
		VALUES ($1, $2, $3)
		RETURNING id, email, password_hash, full_name, created_at, updated_at
	`, u.Email, u.PasswordHash, u.FullName)

	created := &model.User{}
	err := row.Scan(
		&created.ID, &created.Email, &created.PasswordHash,
		&created.FullName, &created.CreatedAt, &created.UpdatedAt,
	)
	if err != nil {
		// pgx surfaces unique constraint violations — check for duplicate email
		if err.Error() == `ERROR: duplicate key value violates unique constraint "users_email_key" (SQLSTATE 23505)` {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	return created, nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, full_name, created_at, updated_at
		FROM users WHERE email = $1
	`, email)

	u := &model.User{}
	err := row.Scan(
		&u.ID, &u.Email, &u.PasswordHash,
		&u.FullName, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	return u, nil
}
func (r *UserRepository) FindByID(ctx context.Context, id string) (*model.User, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, full_name, created_at, updated_at
		FROM users WHERE id = $1
	`, id)

	u := &model.User{}
	err := row.Scan(
		&u.ID, &u.Email, &u.PasswordHash,
		&u.FullName, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return u, nil
}
