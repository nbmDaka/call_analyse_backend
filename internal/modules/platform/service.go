// Package platform contains authorization isolated from ordinary workspace roles.
package platform

import (
	"context"
	"errors"

	"call_analyse_backend/internal/modules/workspaces"

	"github.com/google/uuid"
)

var ErrForbidden = errors.New("platform administration forbidden")

type User struct {
	ID           uuid.UUID               `json:"id"`
	Email        string                  `json:"email"`
	PlatformRole workspaces.PlatformRole `json:"platform_role"`
	Status       string                  `json:"status"`
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

func (s Service) SystemMetrics(ctx context.Context, role workspaces.PlatformRole) (Metrics, error) {
	if err := authorize(role); err != nil || s.store == nil {
		return Metrics{}, ErrForbidden
	}
	return s.store.Metrics(ctx)
}
