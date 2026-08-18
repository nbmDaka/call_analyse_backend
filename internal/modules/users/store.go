// Package users provides focused PostgreSQL access for user records.
package users

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// User is the persisted user record shared by future user-management features.
type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	Role         string
	SupervisorID *uuid.UUID
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Store provides parameterized user lookup and creation queries.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates a PostgreSQL user store.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) FindByEmail(ctx context.Context, email string) (User, error) {
	return scan(s.pool.QueryRow(ctx, `
SELECT id, email, password_hash, role, supervisor_id, created_at, updated_at
FROM users WHERE email = $1`, email))
}

func (s *Store) FindByID(ctx context.Context, id uuid.UUID) (User, error) {
	return scan(s.pool.QueryRow(ctx, `
SELECT id, email, password_hash, role, supervisor_id, created_at, updated_at
FROM users WHERE id = $1`, id))
}

func (s *Store) Create(ctx context.Context, user User) (User, error) {
	return scan(s.pool.QueryRow(ctx, `
INSERT INTO users (id, email, password_hash, role, supervisor_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, email, password_hash, role, supervisor_id, created_at, updated_at`,
		user.ID, user.Email, user.PasswordHash, user.Role, user.SupervisorID))
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scan(row rowScanner) (User, error) {
	var user User
	if err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role, &user.SupervisorID, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return User{}, fmt.Errorf("user not found: %w", err)
		}
		return User{}, err
	}
	return user, nil
}
