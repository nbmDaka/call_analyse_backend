package memberships

import (
	"context"
	"errors"
	"testing"

	"call_analyse_backend/internal/modules/workspaces"
	"github.com/google/uuid"
)

func TestWorkspaceAdminCannotManageOwnerOrAdmin(t *testing.T) {
	workspaceID := uuid.New()
	target := workspaces.Membership{ID: uuid.New(), WorkspaceID: workspaceID, UserID: uuid.New(), Role: workspaces.RoleAdmin, Status: workspaces.MembershipActive}
	store := &fakeStore{target: target}
	service := NewService(store)
	actor := workspaces.Actor{UserID: uuid.New(), WorkspaceID: workspaceID, MembershipID: uuid.New(), WorkspaceRole: workspaces.RoleAdmin, MembershipStatus: workspaces.MembershipActive, WorkspaceStatus: workspaces.StatusActive}
	status := workspaces.MembershipDisabled
	if _, err := service.Update(context.Background(), actor, target.ID, UpdateInput{Status: &status}); !errors.Is(err, workspaces.ErrForbidden) {
		t.Fatalf("Update() error = %v, want forbidden", err)
	}
}

func TestOwnerCannotDisableSelf(t *testing.T) {
	workspaceID, userID, membershipID := uuid.New(), uuid.New(), uuid.New()
	target := workspaces.Membership{ID: membershipID, WorkspaceID: workspaceID, UserID: userID, Role: workspaces.RoleOwner, Status: workspaces.MembershipActive}
	service := NewService(&fakeStore{target: target})
	actor := workspaces.Actor{UserID: userID, WorkspaceID: workspaceID, MembershipID: membershipID, WorkspaceRole: workspaces.RoleOwner, MembershipStatus: workspaces.MembershipActive, WorkspaceStatus: workspaces.StatusActive}
	status := workspaces.MembershipDisabled
	if _, err := service.Update(context.Background(), actor, membershipID, UpdateInput{Status: &status}); !errors.Is(err, workspaces.ErrLastOwner) {
		t.Fatalf("Update() error = %v, want last owner", err)
	}
}

type fakeStore struct{ target workspaces.Membership }

func (s *fakeStore) List(context.Context, workspaces.Actor) ([]workspaces.Membership, error) {
	return []workspaces.Membership{s.target}, nil
}
func (s *fakeStore) Get(_ context.Context, workspaceID, membershipID uuid.UUID) (workspaces.Membership, error) {
	if s.target.WorkspaceID != workspaceID || s.target.ID != membershipID {
		return workspaces.Membership{}, workspaces.ErrMembershipNotFound
	}
	return s.target, nil
}
func (s *fakeStore) CreateByEmail(context.Context, uuid.UUID, CreateInput) (workspaces.Membership, error) {
	return workspaces.Membership{}, nil
}
func (s *fakeStore) Update(context.Context, uuid.UUID, uuid.UUID, UpdateInput) (workspaces.Membership, error) {
	return s.target, nil
}
func (s *fakeStore) Delete(context.Context, uuid.UUID, uuid.UUID) error { return nil }
