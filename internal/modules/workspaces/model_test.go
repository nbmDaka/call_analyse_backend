package workspaces

import (
	"testing"

	"github.com/google/uuid"
)

func TestActorPermissionsAreWorkspaceScoped(t *testing.T) {
	workspaceID := uuid.New()
	manager := Actor{UserID: uuid.New(), WorkspaceID: workspaceID, MembershipID: uuid.New(), WorkspaceRole: RoleManager, MembershipStatus: MembershipActive, WorkspaceStatus: StatusActive}
	if !manager.CanUpload() || !manager.CanViewOwner(manager.UserID) || manager.CanViewOwner(uuid.New()) {
		t.Fatal("manager permissions must be restricted to the manager's own calls")
	}

	admin := Actor{UserID: uuid.New(), WorkspaceID: workspaceID, MembershipID: uuid.New(), WorkspaceRole: RoleAdmin, MembershipStatus: MembershipActive, WorkspaceStatus: StatusActive}
	if !admin.CanViewAllCalls() || !admin.CanManageMembers() {
		t.Fatal("workspace admin must manage members and view workspace calls")
	}
	if admin.PlatformRole == PlatformRoleSuperAdmin {
		t.Fatal("workspace role must not imply platform superadmin")
	}
}

func TestDisabledAndSuspendedActorsFailClosed(t *testing.T) {
	disabled := Actor{UserID: uuid.New(), WorkspaceID: uuid.New(), MembershipID: uuid.New(), WorkspaceRole: RoleOwner, MembershipStatus: MembershipDisabled, WorkspaceStatus: StatusActive}
	if disabled.HasWorkspaceAccess() {
		t.Fatal("disabled membership must not grant workspace access")
	}

	suspended := Actor{UserID: uuid.New(), WorkspaceID: uuid.New(), MembershipID: uuid.New(), WorkspaceRole: RoleOwner, MembershipStatus: MembershipActive, WorkspaceStatus: StatusSuspended}
	if !suspended.HasWorkspaceAccess() || suspended.CanUpload() {
		t.Fatal("suspended workspace remains readable but must reject uploads")
	}
}

func TestPersonalOwnerCannotManageOtherUsers(t *testing.T) {
	actor := Actor{UserID: uuid.New(), WorkspaceID: uuid.New(), MembershipID: uuid.New(), WorkspaceRole: RoleOwner, MembershipStatus: MembershipActive, WorkspaceStatus: StatusActive, WorkspaceType: TypePersonal}
	if actor.CanManageMembers() || actor.CanManageAdmins() {
		t.Fatal("personal workspace owner must not manage other users")
	}
}
