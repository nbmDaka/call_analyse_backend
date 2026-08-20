// Package auth provides password, token, session, and authentication service primitives.
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	RoleAdmin      Role = "admin"
	RoleSupervisor Role = "supervisor"
	RoleManager    Role = "manager"

	// MaxAccessTokenTTL limits non-revocable access tokens to one hour.
	MaxAccessTokenTTL = time.Hour

	accessTokenType  = "access"
	refreshTokenType = "refresh"
	tokenIssuer      = "call-analyse-api"
)

var (
	ErrUserNotFound            = errors.New("user not found")
	ErrEmailAlreadyExists      = errors.New("email already exists")
	ErrInvalidCredentials      = errors.New("invalid credentials")
	ErrEmailNotVerified        = errors.New("email is not verified")
	ErrInvalidActionToken      = errors.New("invalid or expired action token")
	ErrEmailServiceUnavailable = errors.New("email service is unavailable")
	ErrInvalidRefreshToken     = errors.New("invalid refresh token")
	ErrRefreshTokenNotFound    = errors.New("refresh token not found")
)

// Role describes the authorization level granted to a user.
type Role string

// User is the minimal authenticated user record.
type User struct {
	ID              uuid.UUID
	Email           string
	PasswordHash    string
	Role            Role
	SupervisorID    *uuid.UUID
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// PublicUser is the password-free user representation safe for service results.
type PublicUser struct {
	ID            uuid.UUID
	Email         string
	Role          Role
	SupervisorID  *uuid.UUID
	EmailVerified bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ActionTokenPurpose prevents a token issued for one flow from being used in another.
type ActionTokenPurpose string

const (
	ActionTokenEmailVerification ActionTokenPurpose = "email_verification"
	ActionTokenPasswordReset     ActionTokenPurpose = "password_reset"
)

// ActionToken is a one-time, hashed token for an account action.
type ActionToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Purpose   ActionTokenPurpose
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// ActionTokenStore persists one-time account action tokens.
type ActionTokenStore interface {
	CreateActionToken(ctx context.Context, token ActionToken) error
	ConsumeActionToken(ctx context.Context, purpose ActionTokenPurpose, tokenHash string) (uuid.UUID, error)
}

// EmailSender sends transactional email from the backend.
type EmailSender interface {
	Send(ctx context.Context, to, subject, htmlBody, textBody string) error
}

// RefreshToken is the server-side record for a revocable refresh session.
type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

// Claims are the deliberately small JWT payload used for authentication.
type Claims struct {
	UserID    string `json:"user_id"`
	Role      Role   `json:"role"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

// PasswordHasher hashes and verifies user passwords.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(hash, password string) error
}

// TokenManager issues and validates access and refresh tokens.
type TokenManager interface {
	IssueAccess(user User) (string, error)
	IssueRefresh(user User) (string, error)
	ParseAccess(token string) (Claims, error)
	ParseRefresh(token string) (Claims, error)
	HashRefreshToken(token string) string
	VerifyRefreshTokenHash(hash, token string) bool
}

// UserStore is the user persistence boundary required by Service.
type UserStore interface {
	FindByEmail(ctx context.Context, email string) (User, error)
	FindByID(ctx context.Context, id uuid.UUID) (User, error)
	Create(ctx context.Context, user User) (User, error)
	MarkEmailVerified(ctx context.Context, id uuid.UUID) error
	UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error
}

// RefreshTokenStore is the refresh-session persistence boundary required by Service.
type RefreshTokenStore interface {
	Store(ctx context.Context, token RefreshToken) error
	FindActive(ctx context.Context, hash string) (RefreshToken, error)
	Rotate(ctx context.Context, oldHash string, replacement RefreshToken) error
	Revoke(ctx context.Context, hash string) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
}

// TokenPair is returned after successful login and refresh operations.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}
