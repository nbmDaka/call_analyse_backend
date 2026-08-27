package invitations

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

	"call_analyse_backend/internal/modules/auth"
	"call_analyse_backend/internal/modules/workspaces"
	"github.com/google/uuid"
)

const minimumPasswordLength = 8

type Service struct {
	store         Store
	emailer       auth.EmailSender
	hasher        auth.PasswordHasher
	tokens        auth.TokenManager
	frontendURL   string
	invitationTTL time.Duration
}

func NewService(
	store Store,
	emailer auth.EmailSender,
	hasher auth.PasswordHasher,
	tokens auth.TokenManager,
	frontendURL string,
	invitationTTL time.Duration,
) Service {
	if invitationTTL <= 0 {
		invitationTTL = 7 * 24 * time.Hour
	}
	return Service{
		store:         store,
		emailer:       emailer,
		hasher:        hasher,
		tokens:        tokens,
		frontendURL:   strings.TrimRight(strings.TrimSpace(frontendURL), "/"),
		invitationTTL: invitationTTL,
	}
}

func (s Service) Invite(ctx context.Context, actor workspaces.Actor, input CreateInput) (Invitation, error) {
	if !actor.CanManageMembers() || s.store == nil {
		return Invitation{}, workspaces.ErrForbidden
	}
	email := NormalizeEmail(input.Email)
	if email == "" || !strings.Contains(email, "@") {
		return Invitation{}, ErrInvalidEmail
	}
	if input.Role != workspaces.RoleOwner && input.Role != workspaces.RoleAdmin &&
		input.Role != workspaces.RoleSupervisor && input.Role != workspaces.RoleManager {
		return Invitation{}, ErrInvalidRole
	}
	if (input.Role == workspaces.RoleOwner || input.Role == workspaces.RoleAdmin) && !actor.CanManageAdmins() {
		return Invitation{}, workspaces.ErrForbidden
	}

	rawToken, tokenHash, err := newInviteToken()
	if err != nil {
		return Invitation{}, fmt.Errorf("generate invite token: %w", err)
	}

	inv := Invitation{
		ID:              uuid.New(),
		WorkspaceID:     actor.WorkspaceID,
		Email:           email,
		Role:            input.Role,
		InvitedByUserID: actor.UserID,
		ExpiresAt:       time.Now().UTC().Add(s.invitationTTL),
	}

	created, err := s.store.Create(ctx, inv, tokenHash)
	if err != nil {
		return Invitation{}, err
	}

	// Send transactional email
	if s.emailer != nil && s.frontendURL != "" {
		_ = s.sendInviteEmail(ctx, created, rawToken)
	}

	return created, nil
}

func (s Service) ListPending(ctx context.Context, actor workspaces.Actor) ([]Invitation, error) {
	if !actor.CanManageMembers() || s.store == nil {
		return nil, workspaces.ErrForbidden
	}
	return s.store.ListPendingByWorkspace(ctx, actor.WorkspaceID)
}

func (s Service) Revoke(ctx context.Context, actor workspaces.Actor, invitationID uuid.UUID) error {
	if !actor.CanManageMembers() || s.store == nil {
		return workspaces.ErrForbidden
	}
	return s.store.Revoke(ctx, actor.WorkspaceID, invitationID)
}

func (s Service) GetInfo(ctx context.Context, rawToken string) (InvitationInfo, error) {
	if s.store == nil {
		return InvitationInfo{}, ErrInvitationNotFound
	}
	tokenHash := hashInviteToken(rawToken)
	inv, isExistingUser, err := s.store.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return InvitationInfo{}, err
	}
	return InvitationInfo{
		Email:          inv.Email,
		Role:           inv.Role,
		WorkspaceID:    inv.WorkspaceID,
		WorkspaceName:  inv.WorkspaceName,
		IsExistingUser: isExistingUser,
		ExpiresAt:      inv.ExpiresAt,
	}, nil
}

func (s Service) Accept(ctx context.Context, userID uuid.UUID, rawToken string) (uuid.UUID, error) {
	if s.store == nil {
		return uuid.Nil, ErrInvitationNotFound
	}
	tokenHash := hashInviteToken(rawToken)
	return s.store.AcceptForUser(ctx, tokenHash, userID)
}

func (s Service) RegisterAndAccept(ctx context.Context, rawToken, password string) (auth.TokenPair, auth.PublicUser, uuid.UUID, error) {
	if s.store == nil || s.hasher == nil || s.tokens == nil {
		return auth.TokenPair{}, auth.PublicUser{}, uuid.Nil, errors.New("registration service unavailable")
	}
	if strings.TrimSpace(password) == "" {
		return auth.TokenPair{}, auth.PublicUser{}, uuid.Nil, errors.New("password is required")
	}
	if len([]rune(password)) < minimumPasswordLength {
		return auth.TokenPair{}, auth.PublicUser{}, uuid.Nil, fmt.Errorf("password must contain at least %d characters", minimumPasswordLength)
	}

	passwordHash, err := s.hasher.Hash(password)
	if err != nil {
		return auth.TokenPair{}, auth.PublicUser{}, uuid.Nil, fmt.Errorf("hash password: %w", err)
	}

	tokenHash := hashInviteToken(rawToken)
	user, workspaceID, err := s.store.RegisterAndAccept(ctx, tokenHash, passwordHash)
	if err != nil {
		return auth.TokenPair{}, auth.PublicUser{}, uuid.Nil, err
	}

	access, err := s.tokens.IssueAccess(user)
	if err != nil {
		return auth.TokenPair{}, auth.PublicUser{}, uuid.Nil, fmt.Errorf("issue access token: %w", err)
	}
	refresh, err := s.tokens.IssueRefresh(user)
	if err != nil {
		return auth.TokenPair{}, auth.PublicUser{}, uuid.Nil, fmt.Errorf("issue refresh token: %w", err)
	}

	pubUser := auth.PublicUser{
		ID:            user.ID,
		Email:         user.Email,
		PlatformRole:  user.PlatformRole,
		Status:        user.Status,
		Role:          user.Role,
		EmailVerified: user.EmailVerifiedAt != nil,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
	}

	return auth.TokenPair{AccessToken: access, RefreshToken: refresh}, pubUser, workspaceID, nil
}

func (s Service) sendInviteEmail(ctx context.Context, inv Invitation, rawToken string) error {
	link := fmt.Sprintf("%s/invite?token=%s", s.frontendURL, url.QueryEscape(rawToken))
	roleName := roleRussianName(inv.Role)
	wsName := inv.WorkspaceName
	if wsName == "" {
		wsName = "Команда"
	}

	subject := fmt.Sprintf("Приглашение в команду «%s» в Callwise", wsName)
	heading := fmt.Sprintf("Вас пригласили в «%s»", html.EscapeString(wsName))
	body := fmt.Sprintf("Вам предоставлен доступ с ролью <strong>%s</strong> в рабочем пространстве <strong>%s</strong>.", html.EscapeString(roleName), html.EscapeString(wsName))
	btnText := "Принять приглашение"

	htmlBody := fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; line-height: 1.6; color: #1e293b; max-width: 540px; margin: 0 auto; padding: 24px;">
			<div style="margin-bottom: 24px;">
				<h2 style="color: #0f172a; margin-top: 0;">%s</h2>
				<p style="font-size: 15px;">%s</p>
			</div>
			<div style="margin: 28px 0;">
				<a href="%s" style="background-color: #2563eb; color: #ffffff; padding: 12px 24px; text-decoration: none; border-radius: 6px; font-weight: bold; display: inline-block;">%s</a>
			</div>
			<p style="color: #64748b; font-size: 13px; margin-top: 32px; border-top: 1px solid #e2e8f0; padding-top: 16px;">
				Если вы не ожидали это приглашение, просто проигнорируйте данное письмо.
			</p>
		</div>
	`, heading, body, html.EscapeString(link), btnText)

	textBody := fmt.Sprintf("%s\n\n%s\n\nСсылка для входа: %s", heading, body, link)

	return s.emailer.Send(ctx, inv.Email, subject, htmlBody, textBody)
}

func roleRussianName(role workspaces.Role) string {
	switch role {
	case workspaces.RoleOwner:
		return "Владелец"
	case workspaces.RoleAdmin:
		return "Администратор"
	case workspaces.RoleSupervisor:
		return "Супервайзер"
	case workspaces.RoleManager:
		return "Менеджер"
	default:
		return string(role)
	}
}

func newInviteToken() (string, string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	raw := base64.RawURLEncoding.EncodeToString(bytes)
	return raw, hashInviteToken(raw), nil
}

func hashInviteToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
