package workspaces

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type Service struct{ store Store }

func NewService(store Store) Service { return Service{store: store} }

func (s Service) List(ctx context.Context, userID uuid.UUID) ([]AvailableWorkspace, error) {
	if userID == uuid.Nil || s.store == nil {
		return nil, ErrForbidden
	}
	return s.store.ListForUser(ctx, userID)
}

func (s Service) Get(ctx context.Context, userID, workspaceID uuid.UUID) (AvailableWorkspace, error) {
	if userID == uuid.Nil || workspaceID == uuid.Nil || s.store == nil {
		return AvailableWorkspace{}, ErrWorkspaceNotFound
	}
	return s.store.GetForUser(ctx, userID, workspaceID)
}

func (s Service) CreateCompany(ctx context.Context, userID uuid.UUID, name string) (AvailableWorkspace, error) {
	name = strings.TrimSpace(name)
	if userID == uuid.Nil || name == "" || len([]rune(name)) > 160 || s.store == nil {
		return AvailableWorkspace{}, fmt.Errorf("valid company name is required")
	}
	return s.store.CreateCompany(ctx, userID, name)
}

func (s Service) Rename(ctx context.Context, actor Actor, name string) (Workspace, error) {
	name = strings.TrimSpace(name)
	if !actor.HasWorkspaceAccess() || (actor.WorkspaceRole != RoleOwner && actor.WorkspaceRole != RoleAdmin) {
		return Workspace{}, ErrForbidden
	}
	if name == "" || len([]rune(name)) > 160 {
		return Workspace{}, fmt.Errorf("valid workspace name is required")
	}
	return s.store.Rename(ctx, actor.WorkspaceID, name)
}

func (s Service) Delete(ctx context.Context, actor Actor) error {
	if !actor.HasWorkspaceAccess() || actor.WorkspaceRole != RoleOwner {
		return ErrForbidden
	}
	if s.store == nil {
		return ErrForbidden
	}
	return s.store.Delete(ctx, actor.WorkspaceID)
}
