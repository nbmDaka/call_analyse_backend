// Package memberships manages workspace-local roles and supervisor assignments.
package memberships

import (
	"context"

	"call_analyse_backend/internal/modules/workspaces"

	"github.com/google/uuid"
)

type CreateInput struct {
	Email string
	Role  workspaces.Role
}

type UpdateInput struct {
	Role                   *workspaces.Role
	Status                 *workspaces.MembershipStatus
	SupervisorMembershipID *uuid.UUID
	ClearSupervisor        bool
}

type Store interface {
	List(context.Context, workspaces.Actor) ([]workspaces.Membership, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (workspaces.Membership, error)
	CreateByEmail(context.Context, uuid.UUID, CreateInput) (workspaces.Membership, error)
	Update(context.Context, uuid.UUID, uuid.UUID, UpdateInput) (workspaces.Membership, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
}

type Service struct{ store Store }

func NewService(store Store) Service { return Service{store: store} }

func (s Service) List(ctx context.Context, actor workspaces.Actor) ([]workspaces.Membership, error) {
	if !actor.HasWorkspaceAccess() || s.store == nil {
		return nil, workspaces.ErrForbidden
	}
	return s.store.List(ctx, actor)
}

func (s Service) Create(ctx context.Context, actor workspaces.Actor, input CreateInput) (workspaces.Membership, error) {
	if !actor.CanManageMembers() || s.store == nil || !canAssign(actor, input.Role) {
		return workspaces.Membership{}, workspaces.ErrForbidden
	}
	return s.store.CreateByEmail(ctx, actor.WorkspaceID, input)
}

func (s Service) Update(ctx context.Context, actor workspaces.Actor, membershipID uuid.UUID, input UpdateInput) (workspaces.Membership, error) {
	if !actor.CanManageMembers() || s.store == nil {
		return workspaces.Membership{}, workspaces.ErrForbidden
	}
	target, err := s.store.Get(ctx, actor.WorkspaceID, membershipID)
	if err != nil {
		return workspaces.Membership{}, err
	}
	if !canManageTarget(actor, target) || (input.Role != nil && !canAssign(actor, *input.Role)) {
		return workspaces.Membership{}, workspaces.ErrForbidden
	}
	if target.Role == workspaces.RoleOwner && target.UserID == actor.UserID && input.Status != nil && *input.Status != workspaces.MembershipActive {
		return workspaces.Membership{}, workspaces.ErrLastOwner
	}
	return s.store.Update(ctx, actor.WorkspaceID, membershipID, input)
}

func (s Service) Delete(ctx context.Context, actor workspaces.Actor, membershipID uuid.UUID) error {
	if !actor.CanManageMembers() || s.store == nil {
		return workspaces.ErrForbidden
	}
	target, err := s.store.Get(ctx, actor.WorkspaceID, membershipID)
	if err != nil {
		return err
	}
	if !canManageTarget(actor, target) || (target.Role == workspaces.RoleOwner && target.UserID == actor.UserID) {
		return workspaces.ErrLastOwner
	}
	return s.store.Delete(ctx, actor.WorkspaceID, membershipID)
}

func canAssign(actor workspaces.Actor, role workspaces.Role) bool {
	if role == workspaces.RoleOwner || role == workspaces.RoleAdmin {
		return actor.CanManageAdmins()
	}
	return role == workspaces.RoleSupervisor || role == workspaces.RoleManager
}

func canManageTarget(actor workspaces.Actor, target workspaces.Membership) bool {
	if target.WorkspaceID != actor.WorkspaceID {
		return false
	}
	if target.Role == workspaces.RoleOwner || target.Role == workspaces.RoleAdmin {
		return actor.CanManageAdmins()
	}
	return true
}
