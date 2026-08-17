package middleware

import "call_analyse_backend/internal/auth"

// CanViewCall is a pure ownership policy for call-level access checks.
// Supervisor-to-manager relationship scoping belongs in the query that selects managers.
func CanViewCall(role auth.Role, authenticatedUserID, managerID string) bool {
	switch role {
	case auth.RoleAdmin, auth.RoleSupervisor:
		return true
	case auth.RoleManager:
		return authenticatedUserID != "" && authenticatedUserID == managerID
	default:
		return false
	}
}
