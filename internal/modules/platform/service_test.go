package platform

import (
	"context"
	"errors"
	"testing"

	"call_analyse_backend/internal/modules/workspaces"
	"github.com/google/uuid"
)

func TestWorkspaceRoleCannotGrantPlatformAdministration(t *testing.T) {
	service := NewService(nil)
	if _, err := service.ListUsers(context.Background(), workspaces.PlatformRoleUser); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ListUsers() error = %v, want platform forbidden", err)
	}
}

func TestSuperAdminCannotSuspendSelf(t *testing.T) {
	userID := uuid.New()
	service := NewService(nil)
	if _, err := service.SetUserStatus(context.Background(), userID, workspaces.PlatformRoleSuperAdmin, userID, "suspended"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("SetUserStatus(self) error = %v, want forbidden", err)
	}
}
