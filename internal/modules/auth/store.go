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
SELECT id, email, password_hash, role, supervisor_id, email_verified_at, created_at, updated_at
FROM users WHERE email = $1`, email))
}

func (s *PostgresStore) FindByID(ctx context.Context, id uuid.UUID) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `
SELECT id, email, password_hash, role, supervisor_id, email_verified_at, created_at, updated_at
FROM users WHERE id = $1`, id))
}

func (s *PostgresStore) Create(ctx context.Context, user User) (User, error) {
	created, err := scanUser(s.pool.QueryRow(ctx, `
INSERT INTO users (id, email, password_hash, role, supervisor_id, email_verified_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, email, password_hash, role, supervisor_id, email_verified_at, created_at, updated_at`,
		user.ID, user.Email, user.PasswordHash, user.Role, user.SupervisorID, user.EmailVerifiedAt))
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

func (s *PostgresStore) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
UPDATE refresh_tokens SET revoked_at = NOW()
WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

func (s *PostgresStore) MarkEmailVerified(ctx context.Context, id uuid.UUID) error {
	result, err := s.pool.Exec(ctx, `UPDATE users SET email_verified_at = COALESCE(email_verified_at, NOW()), updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (s *PostgresStore) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	result, err := s.pool.Exec(ctx, `UPDATE users SET password_hash = $2, updated_at = NOW() WHERE id = $1`, id, passwordHash)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (s *PostgresStore) CreateActionToken(ctx context.Context, token ActionToken) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin action token creation: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM auth_action_tokens WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL`, token.UserID, token.Purpose); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO auth_action_tokens (id, user_id, purpose, token_hash, expires_at)
VALUES ($1, $2, $3, $4, $5)`, token.ID, token.UserID, token.Purpose, token.TokenHash, token.ExpiresAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) ConsumeActionToken(ctx context.Context, purpose ActionTokenPurpose, tokenHash string) (uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)
	var id, userID uuid.UUID
	err = tx.QueryRow(ctx, `
SELECT id, user_id FROM auth_action_tokens
WHERE purpose = $1 AND token_hash = $2 AND used_at IS NULL AND expires_at > NOW()
FOR UPDATE`, purpose, tokenHash).Scan(&id, &userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrInvalidActionToken
	}
	if err != nil {
		return uuid.Nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE auth_action_tokens SET used_at = NOW() WHERE id = $1`, id); err != nil {
		return uuid.Nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (User, error) {
	var user User
	var role string
	err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &role, &user.SupervisorID, &user.EmailVerifiedAt, &user.CreatedAt, &user.UpdatedAt)
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
