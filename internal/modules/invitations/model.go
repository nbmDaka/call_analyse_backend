// Package invitations manages workspace email invitation tokens and acceptance flows.
package invitations

import (
	"errors"
	"strings"
	"time"

	"call_analyse_backend/internal/modules/workspaces"
	"github.com/google/uuid"
)

var (
	ErrInvitationNotFound = errors.New("invitation not found")
	ErrInvitationExpired  = errors.New("invitation has expired")
	ErrInvitationAccepted = errors.New("invitation already accepted")
	ErrInvalidEmail       = errors.New("invalid email address")
	ErrAlreadyMember      = errors.New("user is already a member of this workspace")
	ErrInvalidRole        = errors.New("invalid workspace role for invitation")
)

type Invitation struct {
	ID              uuid.UUID       `json:"id"`
	WorkspaceID     uuid.UUID       `json:"workspace_id"`
	WorkspaceName   string          `json:"workspace_name,omitempty"`
	Email           string          `json:"email"`
	Role            workspaces.Role `json:"role"`
	InvitedByUserID uuid.UUID       `json:"invited_by_user_id"`
	ExpiresAt       time.Time       `json:"expires_at"`
	AcceptedAt      *time.Time      `json:"accepted_at,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type InvitationInfo struct {
	Email          string          `json:"email"`
	Role           workspaces.Role `json:"role"`
	WorkspaceID    uuid.UUID       `json:"workspace_id"`
	WorkspaceName  string          `json:"workspace_name"`
	IsExistingUser bool            `json:"is_existing_user"`
	ExpiresAt      time.Time       `json:"expires_at"`
}

type CreateInput struct {
	Email string          `json:"email"`
	Role  workspaces.Role `json:"role"`
}

type RegisterByInviteInput struct {
	Password string `json:"password"`
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
