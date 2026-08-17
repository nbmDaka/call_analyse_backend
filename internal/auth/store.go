package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore persists auth users and refresh sessions through parameterized pgx queries.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates an auth store backed by a pgx pool.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) FindByEmail(ctx context.Context, email string) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `
SELECT id, email, password_hash, role, supervisor_id, created_at, updated_at
FROM users WHERE email = $1`, email))
}

func (s *PostgresStore) FindByID(ctx context.Context, id uuid.UUID) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `
SELECT id, email, password_hash, role, supervisor_id, created_at, updated_at
FROM users WHERE id = $1`, id))
}

func (s *PostgresStore) Create(ctx context.Context, user User) (User, error) {
	created, err := scanUser(s.pool.QueryRow(ctx, `
INSERT INTO users (id, email, password_hash, role, supervisor_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, email, password_hash, role, supervisor_id, created_at, updated_at`,
		user.ID, user.Email, user.PasswordHash, user.Role, user.SupervisorID))
	if isUniqueViolation(err) {
		return User{}, ErrEmailAlreadyExists
	}
	return created, err
}

func (s *PostgresStore) Store(ctx context.Context, token RefreshToken) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4)`, token.ID, token.UserID, token.TokenHash, token.ExpiresAt)
	return err
}

func (s *PostgresStore) FindActive(ctx context.Context, hash string) (RefreshToken, error) {
	var token RefreshToken
	err := s.pool.QueryRow(ctx, `
SELECT id, user_id, token_hash, expires_at, revoked_at, created_at
FROM refresh_tokens
WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()`, hash).
		Scan(&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.RevokedAt, &token.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return RefreshToken{}, ErrRefreshTokenNotFound
	}
	return token, err
}

// Rotate revokes the current active session and inserts its replacement in one transaction.
func (s *PostgresStore) Rotate(ctx context.Context, oldHash string, replacement RefreshToken) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin refresh token rotation: %w", err)
	}
	defer tx.Rollback(ctx)

	var id uuid.UUID
	err = tx.QueryRow(ctx, `
SELECT id FROM refresh_tokens
WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()
FOR UPDATE`, oldHash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrRefreshTokenNotFound
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = NOW() WHERE id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4)`, replacement.ID, replacement.UserID, replacement.TokenHash, replacement.ExpiresAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) Revoke(ctx context.Context, hash string) error {
	result, err := s.pool.Exec(ctx, `
UPDATE refresh_tokens SET revoked_at = NOW()
WHERE token_hash = $1 AND revoked_at IS NULL`, hash)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrRefreshTokenNotFound
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (User, error) {
	var user User
	var role string
	err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &role, &user.SupervisorID, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	user.Role = Role(role)
	return user, err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
