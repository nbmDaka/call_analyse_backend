// Package platform contains authorization isolated from ordinary workspace roles.
package platform

import (
	"context"
	"errors"
	"time"

	"call_analyse_backend/internal/modules/workspaces"

	"github.com/google/uuid"
)

var ErrForbidden = errors.New("platform administration forbidden")

type User struct {
	ID           uuid.UUID               `json:"id"`
	Email        string                  `json:"email"`
	PlatformRole workspaces.PlatformRole `json:"platform_role"`
	Status       string                  `json:"status"`
	CreatedAt    time.Time               `json:"created_at"`
}

type CallSummary struct {
	ID               uuid.UUID `json:"id"`
	WorkspaceID      uuid.UUID `json:"workspace_id"`
	WorkspaceName    string    `json:"workspace_name"`
	WorkspaceType    string    `json:"workspace_type"`
	OwnerUserID      uuid.UUID `json:"owner_user_id"`
	OwnerEmail       string    `json:"owner_email"`
	OriginalFilename string    `json:"original_filename"`
	Status           string    `json:"status"`
	SizeBytes        int64     `json:"size_bytes"`
	DurationSeconds  *int      `json:"duration_seconds,omitempty"`
	TotalScore       *int      `json:"total_score,omitempty"`
	ErrorMessage     *string   `json:"error_message,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type CallListFilter struct {
	Status *string
	Limit  int
	Offset int
}

type CallListPage struct {
	Calls      []CallSummary `json:"calls"`
	Total      int           `json:"total"`
	Page       int           `json:"page"`
	PerPage    int           `json:"per_page"`
	TotalPages int           `json:"total_pages"`
}

type Metrics struct {
	Users      int `json:"users"`
	Workspaces int `json:"workspaces"`
	Calls      int `json:"calls"`
}

type Store interface {
	CreateCompany(context.Context, uuid.UUID, uuid.UUID, string) (workspaces.Workspace, error)
	ListWorkspaces(context.Context, *workspaces.Type) ([]workspaces.Workspace, error)
	ListUsers(context.Context) ([]User, error)
	SetWorkspaceStatus(context.Context, uuid.UUID, uuid.UUID, workspaces.Status) (workspaces.Workspace, error)
	SetUserStatus(context.Context, uuid.UUID, uuid.UUID, string) (User, error)
	SetUserPlatformRole(context.Context, uuid.UUID, uuid.UUID, workspaces.PlatformRole) (User, error)
	ListCalls(context.Context, CallListFilter) (CallListPage, error)
	Metrics(context.Context) (Metrics, error)
}

type Service struct{ store Store }

func NewService(store Store) Service { return Service{store: store} }

func authorize(role workspaces.PlatformRole) error {
	if role != workspaces.PlatformRoleSuperAdmin {
		return ErrForbidden
	}
	return nil
}

func (s Service) ListWorkspaces(ctx context.Context, role workspaces.PlatformRole, workspaceType *workspaces.Type) ([]workspaces.Workspace, error) {
	if err := authorize(role); err != nil || s.store == nil {
		return nil, ErrForbidden
	}
	return s.store.ListWorkspaces(ctx, workspaceType)
}

func (s Service) CreateCompany(ctx context.Context, actorUserID uuid.UUID, role workspaces.PlatformRole, ownerUserID uuid.UUID, name string) (workspaces.Workspace, error) {
	if err := authorize(role); err != nil || s.store == nil || ownerUserID == uuid.Nil || name == "" {
		return workspaces.Workspace{}, ErrForbidden
	}
	return s.store.CreateCompany(ctx, actorUserID, ownerUserID, name)
}

func (s Service) ListUsers(ctx context.Context, role workspaces.PlatformRole) ([]User, error) {
	if err := authorize(role); err != nil || s.store == nil {
		return nil, ErrForbidden
	}
	return s.store.ListUsers(ctx)
}

func (s Service) SetWorkspaceStatus(ctx context.Context, actorUserID uuid.UUID, role workspaces.PlatformRole, workspaceID uuid.UUID, status workspaces.Status) (workspaces.Workspace, error) {
	if err := authorize(role); err != nil || s.store == nil {
		return workspaces.Workspace{}, ErrForbidden
	}
	if status != workspaces.StatusActive && status != workspaces.StatusSuspended {
		return workspaces.Workspace{}, errors.New("invalid workspace status")
	}
	return s.store.SetWorkspaceStatus(ctx, actorUserID, workspaceID, status)
}

func (s Service) SetUserStatus(ctx context.Context, actorUserID uuid.UUID, role workspaces.PlatformRole, userID uuid.UUID, status string) (User, error) {
	if err := authorize(role); err != nil || s.store == nil || userID == actorUserID {
		return User{}, ErrForbidden
	}
	if status != "active" && status != "suspended" {
		return User{}, errors.New("invalid user status")
	}
	return s.store.SetUserStatus(ctx, actorUserID, userID, status)
}

func (s Service) SetUserPlatformRole(ctx context.Context, actorUserID uuid.UUID, role workspaces.PlatformRole, targetUserID uuid.UUID, targetRole workspaces.PlatformRole) (User, error) {
	if err := authorize(role); err != nil || s.store == nil || targetUserID == actorUserID {
		return User{}, ErrForbidden
	}
	if targetRole != workspaces.PlatformRoleUser && targetRole != workspaces.PlatformRoleSuperAdmin {
		return User{}, errors.New("invalid platform role")
	}
	return s.store.SetUserPlatformRole(ctx, actorUserID, targetUserID, targetRole)
}

func (s Service) ListCalls(ctx context.Context, role workspaces.PlatformRole, filter CallListFilter) (CallListPage, error) {
	if err := authorize(role); err != nil || s.store == nil {
		return CallListPage{}, ErrForbidden
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.store.ListCalls(ctx, filter)
}

func (s Service) SystemMetrics(ctx context.Context, role workspaces.PlatformRole) (Metrics, error) {
	if err := authorize(role); err != nil || s.store == nil {
		return Metrics{}, ErrForbidden
	}
	return s.store.Metrics(ctx)
}
