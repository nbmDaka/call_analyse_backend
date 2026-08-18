package middleware

import (
	"testing"

	"call_analyse_backend/internal/modules/auth"
)

func TestCanViewCallManagerCannotViewAnotherManagersCall(t *testing.T) {
	if CanViewCall(auth.RoleManager, "manager-one", "manager-two") {
		t.Fatal("CanViewCall() = true, want false for another manager's call")
	}
}

func TestCanViewCallSupervisorCanViewManagerCall(t *testing.T) {
	if !CanViewCall(auth.RoleSupervisor, "supervisor-one", "manager-one") {
		t.Fatal("CanViewCall() = false, want true for a supervisor")
	}
}

func TestCanViewCallAdminCanViewAllCalls(t *testing.T) {
	if !CanViewCall(auth.RoleAdmin, "admin-one", "manager-one") {
		t.Fatal("CanViewCall() = false, want true for an admin")
	}
}
