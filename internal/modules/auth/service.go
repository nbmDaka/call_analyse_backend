package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Service implements login, refresh rotation, logout, current-user lookup, and bootstrap admin creation.
type Service struct {
	users    UserStore
	refresh  RefreshTokenStore
	password PasswordHasher
	tokens   TokenManager
}

// NewService constructs an authentication service from focused persistence and crypto boundaries.
func NewService(users UserStore, refresh RefreshTokenStore, password PasswordHasher, tokens TokenManager) Service {
	return Service{users: users, refresh: refresh, password: password, tokens: tokens}
}

// Login verifies credentials and creates a new revocable refresh session.
func (s Service) Login(ctx context.Context, email, password string) (TokenPair, error) {
	user, err := s.users.FindByEmail(ctx, normalizeEmail(email))
	if err != nil || s.password.Verify(user.PasswordHash, password) != nil {
		return TokenPair{}, ErrInvalidCredentials
	}
	return s.issuePair(ctx, user, "")
}

// Register creates a new manager user and returns a token pair upon success.
func (s Service) Register(ctx context.Context, email, password string) (TokenPair, error) {
	email = normalizeEmail(email)
	if email == "" || password == "" {
		return TokenPair{}, errors.New("email and password are required")
	}
	hash, err := s.password.Hash(password)
	if err != nil {
		return TokenPair{}, fmt.Errorf("hash password: %w", err)
	}
	user, err := s.users.Create(ctx, User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: hash,
		Role:         RoleManager,
	})
	if err != nil {
		return TokenPair{}, err
	}
	return s.issuePair(ctx, user, "")
}


// Refresh validates the refresh JWT and atomically replaces its server-side session.
func (s Service) Refresh(ctx context.Context, rawToken string) (TokenPair, error) {
	claims, err := s.tokens.ParseRefresh(rawToken)
	if err != nil {
		return TokenPair{}, ErrInvalidRefreshToken
	}
	hash := s.tokens.HashRefreshToken(rawToken)
	stored, err := s.refresh.FindActive(ctx, hash)
	if err != nil || !s.tokens.VerifyRefreshTokenHash(stored.TokenHash, rawToken) {
		return TokenPair{}, ErrInvalidRefreshToken
	}
	userID, err := uuid.Parse(claims.UserID)
	if err != nil || stored.UserID != userID {
		return TokenPair{}, ErrInvalidRefreshToken
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return TokenPair{}, ErrInvalidRefreshToken
	}
	return s.issuePair(ctx, user, hash)
}

// Logout revokes an active server-side refresh session.
func (s Service) Logout(ctx context.Context, rawToken string) error {
	if _, err := s.tokens.ParseRefresh(rawToken); err != nil {
		return ErrInvalidRefreshToken
	}
	if err := s.refresh.Revoke(ctx, s.tokens.HashRefreshToken(rawToken)); err != nil {
		return ErrInvalidRefreshToken
	}
	return nil
}

// Me returns the current user by authenticated ID.
func (s Service) Me(ctx context.Context, userID uuid.UUID) (PublicUser, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return PublicUser{}, err
	}
	return toPublicUser(user), nil
}

// BootstrapAdmin creates the configured admin only when its email does not already exist.
func (s Service) BootstrapAdmin(ctx context.Context, email, password string) (PublicUser, bool, error) {
	email = normalizeEmail(email)
	if email == "" || password == "" {
		return PublicUser{}, false, fmt.Errorf("bootstrap admin email and password are required")
	}
	existing, err := s.users.FindByEmail(ctx, email)
	if err == nil {
		return toPublicUser(existing), false, nil
	}
	if !errors.Is(err, ErrUserNotFound) {
		return PublicUser{}, false, err
	}
	hash, err := s.password.Hash(password)
	if err != nil {
		return PublicUser{}, false, fmt.Errorf("hash bootstrap admin password: %w", err)
	}
	created, err := s.users.Create(ctx, User{ID: uuid.New(), Email: email, PasswordHash: hash, Role: RoleAdmin})
	if errors.Is(err, ErrEmailAlreadyExists) {
		existing, lookupErr := s.users.FindByEmail(ctx, email)
		if lookupErr != nil {
			return PublicUser{}, false, lookupErr
		}
		return toPublicUser(existing), false, nil
	}
	if err != nil {
		return PublicUser{}, false, err
	}
	return toPublicUser(created), true, nil
}

func (s Service) issuePair(ctx context.Context, user User, oldHash string) (TokenPair, error) {
	access, err := s.tokens.IssueAccess(user)
	if err != nil {
		return TokenPair{}, err
	}
	refresh, err := s.tokens.IssueRefresh(user)
	if err != nil {
		return TokenPair{}, err
	}
	claims, err := s.tokens.ParseRefresh(refresh)
	if err != nil || claims.ExpiresAt == nil {
		return TokenPair{}, ErrInvalidRefreshToken
	}
	record := RefreshToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: s.tokens.HashRefreshToken(refresh),
		ExpiresAt: claims.ExpiresAt.Time,
	}
	if oldHash == "" {
		err = s.refresh.Store(ctx, record)
	} else {
		err = s.refresh.Rotate(ctx, oldHash, record)
	}
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func toPublicUser(user User) PublicUser {
	return PublicUser{
		ID:           user.ID,
		Email:        user.Email,
		Role:         user.Role,
		SupervisorID: user.SupervisorID,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}
}
