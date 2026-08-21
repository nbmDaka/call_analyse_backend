package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service implements authentication and account email flows.
type Service struct {
	users             UserStore
	refresh           RefreshTokenStore
	password          PasswordHasher
	tokens            TokenManager
	actionTokens      ActionTokenStore
	emailer           EmailSender
	frontendURL       string
	verificationTTL   time.Duration
	passwordResetTTL  time.Duration
	emailFlowsEnabled bool
}

const minimumPasswordLength = 8

// NewService constructs the authentication service used by unit tests and legacy callers.
// Email flows are enabled by NewServiceWithEmail.
func NewService(users UserStore, refresh RefreshTokenStore, password PasswordHasher, tokens TokenManager) Service {
	return Service{users: users, refresh: refresh, password: password, tokens: tokens}
}

// NewServiceWithEmail enables email verification and password reset flows.
func NewServiceWithEmail(users UserStore, refresh RefreshTokenStore, password PasswordHasher, tokens TokenManager, actionTokens ActionTokenStore, mailer EmailSender, frontendURL string, verificationTTL, passwordResetTTL time.Duration) Service {
	if verificationTTL <= 0 {
		verificationTTL = 24 * time.Hour
	}
	if passwordResetTTL <= 0 {
		passwordResetTTL = time.Hour
	}
	return Service{
		users:             users,
		refresh:           refresh,
		password:          password,
		tokens:            tokens,
		actionTokens:      actionTokens,
		emailer:           mailer,
		frontendURL:       strings.TrimRight(strings.TrimSpace(frontendURL), "/"),
		verificationTTL:   verificationTTL,
		passwordResetTTL:  passwordResetTTL,
		emailFlowsEnabled: true,
	}
}

// Login verifies credentials and creates a new revocable refresh session.
func (s Service) Login(ctx context.Context, email, password string) (TokenPair, error) {
	user, err := s.users.FindByEmail(ctx, normalizeEmail(email))
	if err != nil || s.password.Verify(user.PasswordHash, password) != nil {
		return TokenPair{}, ErrInvalidCredentials
	}
	if user.EmailVerifiedAt == nil {
		return TokenPair{}, ErrEmailNotVerified
	}
	if user.Status == "suspended" {
		return TokenPair{}, ErrUserSuspended
	}
	return s.issuePair(ctx, user, "")
}

// Register creates a manager user and sends an email verification link.
func (s Service) Register(ctx context.Context, email, password string) error {
	email = normalizeEmail(email)
	if email == "" || password == "" {
		return errors.New("email and password are required")
	}
	if len([]rune(password)) < minimumPasswordLength {
		return fmt.Errorf("password must contain at least %d characters", minimumPasswordLength)
	}
	if s.emailFlowsEnabled && (s.actionTokens == nil || s.emailer == nil || s.frontendURL == "") {
		return ErrEmailServiceUnavailable
	}
	hash, err := s.password.Hash(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	var verifiedAt *time.Time
	if !s.emailFlowsEnabled {
		now := time.Now().UTC()
		verifiedAt = &now
	}
	candidate := User{
		ID:              uuid.New(),
		Email:           email,
		PasswordHash:    hash,
		Role:            RoleManager,
		PlatformRole:    PlatformRoleUser,
		Status:          "active",
		EmailVerifiedAt: verifiedAt,
	}
	var user User
	if registrar, ok := s.users.(RegistrationStore); ok {
		user, err = registrar.CreateWithPersonalWorkspace(ctx, candidate, personalWorkspaceName(email))
	} else {
		user, err = s.users.Create(ctx, candidate)
	}
	if err != nil {
		if errors.Is(err, ErrEmailAlreadyExists) && s.emailFlowsEnabled {
			existing, lookupErr := s.users.FindByEmail(ctx, email)
			if lookupErr == nil && existing.EmailVerifiedAt == nil {
				return s.sendActionEmail(ctx, existing, ActionTokenEmailVerification)
			}
		}
		return err
	}
	if !s.emailFlowsEnabled {
		return nil
	}
	return s.sendActionEmail(ctx, user, ActionTokenEmailVerification)
}

// ConfirmEmail consumes a verification token and marks the account verified.
func (s Service) ConfirmEmail(ctx context.Context, rawToken string) error {
	if !s.emailFlowsEnabled || s.actionTokens == nil {
		return ErrEmailServiceUnavailable
	}
	userID, err := s.actionTokens.ConsumeActionToken(ctx, ActionTokenEmailVerification, hashActionToken(rawToken))
	if err != nil {
		return ErrInvalidActionToken
	}
	if err := s.users.MarkEmailVerified(ctx, userID); err != nil {
		return err
	}
	return nil
}

// ResendVerification sends a fresh verification link when the account exists and is not verified.
func (s Service) ResendVerification(ctx context.Context, email string) error {
	if !s.emailFlowsEnabled {
		return ErrEmailServiceUnavailable
	}
	user, err := s.users.FindByEmail(ctx, normalizeEmail(email))
	if errors.Is(err, ErrUserNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if user.EmailVerifiedAt != nil {
		return nil
	}
	return s.sendActionEmail(ctx, user, ActionTokenEmailVerification)
}

// RequestPasswordReset sends a reset link without revealing whether an email exists.
func (s Service) RequestPasswordReset(ctx context.Context, email string) error {
	if !s.emailFlowsEnabled {
		return ErrEmailServiceUnavailable
	}
	user, err := s.users.FindByEmail(ctx, normalizeEmail(email))
	if errors.Is(err, ErrUserNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.sendActionEmail(ctx, user, ActionTokenPasswordReset)
}

// ResetPassword consumes a reset token, changes the password, and revokes active sessions.
func (s Service) ResetPassword(ctx context.Context, rawToken, password string) error {
	if !s.emailFlowsEnabled || s.actionTokens == nil {
		return ErrEmailServiceUnavailable
	}
	if strings.TrimSpace(password) == "" {
		return errors.New("password is required")
	}
	if len([]rune(password)) < minimumPasswordLength {
		return fmt.Errorf("password must contain at least %d characters", minimumPasswordLength)
	}
	hash, err := s.password.Hash(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	userID, err := s.actionTokens.ConsumeActionToken(ctx, ActionTokenPasswordReset, hashActionToken(rawToken))
	if err != nil {
		return ErrInvalidActionToken
	}
	if err := s.users.UpdatePassword(ctx, userID, hash); err != nil {
		return err
	}
	return s.refresh.RevokeAllForUser(ctx, userID)
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
	if user.Status == "suspended" {
		return TokenPair{}, ErrUserSuspended
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
		if existing.PlatformRole != PlatformRoleSuperAdmin {
			if roles, ok := s.users.(PlatformRoleStore); ok {
				existing, err = roles.SetPlatformRole(ctx, existing.ID, PlatformRoleSuperAdmin)
				if err != nil {
					return PublicUser{}, false, err
				}
			}
		}
		return toPublicUser(existing), false, nil
	}
	if !errors.Is(err, ErrUserNotFound) {
		return PublicUser{}, false, err
	}
	hash, err := s.password.Hash(password)
	if err != nil {
		return PublicUser{}, false, fmt.Errorf("hash bootstrap admin password: %w", err)
	}
	now := time.Now().UTC()
	created, err := s.users.Create(ctx, User{ID: uuid.New(), Email: email, PasswordHash: hash, Role: RoleManager, PlatformRole: PlatformRoleSuperAdmin, Status: "active", EmailVerifiedAt: &now})
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

func (s Service) sendActionEmail(ctx context.Context, user User, purpose ActionTokenPurpose) error {
	rawToken, tokenHash, err := newActionToken()
	if err != nil {
		return fmt.Errorf("generate action token: %w", err)
	}
	ttl := s.verificationTTL
	path := "/verify-email"
	subject := "Подтвердите регистрацию в Call Analyse"
	heading := "Подтвердите регистрацию"
	body := "Перейдите по ссылке, чтобы подтвердить адрес электронной почты и войти в Call Analyse."
	if purpose == ActionTokenPasswordReset {
		ttl = s.passwordResetTTL
		path = "/reset-password"
		subject = "Сброс пароля в Call Analyse"
		heading = "Сброс пароля"
		body = "Перейдите по ссылке, чтобы задать новый пароль. Ссылка действует ограниченное время."
	}
	if err := s.actionTokens.CreateActionToken(ctx, ActionToken{ID: uuid.New(), UserID: user.ID, Purpose: purpose, TokenHash: tokenHash, ExpiresAt: time.Now().UTC().Add(ttl)}); err != nil {
		return fmt.Errorf("store action token: %w", err)
	}
	link := s.frontendURL + path + "?token=" + url.QueryEscape(rawToken)
	address := html.EscapeString(user.Email)
	htmlBody := fmt.Sprintf(`<div style="font-family:Arial,sans-serif;line-height:1.5"><h2>%s</h2><p>%s</p><p><a href="%s">Продолжить</a></p><p style="color:#666;font-size:12px">Если вы не запрашивали это действие для %s, просто проигнорируйте письмо.</p></div>`, heading, body, html.EscapeString(link), address)
	textBody := fmt.Sprintf("%s\n\n%s\n%s", heading, body, link)
	if err := s.emailer.Send(ctx, user.Email, subject, htmlBody, textBody); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}

func newActionToken() (string, string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	raw := base64.RawURLEncoding.EncodeToString(bytes)
	return raw, hashActionToken(raw), nil
}

func hashActionToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func personalWorkspaceName(email string) string {
	name := strings.TrimSpace(strings.SplitN(email, "@", 2)[0])
	if name == "" {
		name = "Personal"
	}
	return name + "'s workspace"
}

func toPublicUser(user User) PublicUser {
	return PublicUser{
		ID:            user.ID,
		Email:         user.Email,
		PlatformRole:  platformRole(user.PlatformRole),
		Status:        userStatus(user.Status),
		Role:          user.Role,
		SupervisorID:  user.SupervisorID,
		EmailVerified: user.EmailVerifiedAt != nil,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
	}
}
