// Package workspaces defines tenant identity, membership roles, and authorization actors.
package workspaces

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type PlatformRole string

const (
	PlatformRoleUser       PlatformRole = "user"
	PlatformRoleSuperAdmin PlatformRole = "super_admin"
)

type Type string

const (
	TypePersonal Type = "personal"
	TypeCompany  Type = "company"
)

type Status string

const (
	StatusActive    Status = "active"
	StatusSuspended Status = "suspended"
)

type Role string

const (
	RoleOwner      Role = "owner"
	RoleAdmin      Role = "admin"
	RoleSupervisor Role = "supervisor"
	RoleManager    Role = "manager"
)

type MembershipStatus string

const (
	MembershipInvited  MembershipStatus = "invited"
	MembershipActive   MembershipStatus = "active"
	MembershipDisabled MembershipStatus = "disabled"
)

var (
	ErrWorkspaceNotFound  = errors.New("workspace not found")
	ErrMembershipNotFound = errors.New("membership not found")
	ErrWorkspaceSuspended = errors.New("workspace is suspended")
	ErrMembershipDisabled = errors.New("membership is disabled")
	ErrForbidden          = errors.New("workspace access forbidden")
	ErrLastOwner          = errors.New("the last workspace owner cannot be removed or disabled")
	ErrInvalidSupervisor  = errors.New("supervisor must be an active supervisor in the same workspace")
)

type Workspace struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Type        Type      `json:"type"`
	Status      Status    `json:"status"`
	OwnerUserID uuid.UUID `json:"owner_user_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Membership struct {
	ID                     uuid.UUID        `json:"id"`
	WorkspaceID            uuid.UUID        `json:"workspace_id"`
	UserID                 uuid.UUID        `json:"user_id"`
	Email                  string           `json:"email,omitempty"`
	Role                   Role             `json:"role"`
	Status                 MembershipStatus `json:"status"`
	SupervisorMembershipID *uuid.UUID       `json:"supervisor_membership_id,omitempty"`
	CreatedAt              time.Time        `json:"created_at"`
	UpdatedAt              time.Time        `json:"updated_at"`
}

type AvailableWorkspace struct {
	Workspace
	MembershipID     uuid.UUID        `json:"membership_id"`
	MembershipRole   Role             `json:"membership_role"`
	MembershipStatus MembershipStatus `json:"membership_status"`
}

// Actor is rebuilt from current database state for every workspace request.
type Actor struct {
	UserID           uuid.UUID
	WorkspaceID      uuid.UUID
	MembershipID     uuid.UUID
	WorkspaceRole    Role
	PlatformRole     PlatformRole
	MembershipStatus MembershipStatus
	WorkspaceStatus  Status
	WorkspaceType    Type
}

func (a Actor) HasWorkspaceAccess() bool {
	return a.UserID != uuid.Nil && a.WorkspaceID != uuid.Nil && a.MembershipID != uuid.Nil &&
		a.MembershipStatus == MembershipActive &&
		(a.WorkspaceRole == RoleOwner || a.WorkspaceRole == RoleAdmin || a.WorkspaceRole == RoleSupervisor || a.WorkspaceRole == RoleManager)
}

func (a Actor) CanUpload() bool {
	if !a.HasWorkspaceAccess() || a.WorkspaceStatus != StatusActive {
		return false
	}
	return a.WorkspaceRole == RoleOwner || a.WorkspaceRole == RoleAdmin || a.WorkspaceRole == RoleManager
}

func (a Actor) CanViewAllCalls() bool {
	return a.HasWorkspaceAccess() && (a.WorkspaceRole == RoleOwner || a.WorkspaceRole == RoleAdmin)
}

func (a Actor) CanManageMembers() bool {
	return a.WorkspaceType != TypePersonal && a.HasWorkspaceAccess() && (a.WorkspaceRole == RoleOwner || a.WorkspaceRole == RoleAdmin)
}

func (a Actor) CanManageAdmins() bool {
	return a.WorkspaceType != TypePersonal && a.HasWorkspaceAccess() && a.WorkspaceRole == RoleOwner
}

func (a Actor) CanViewOwner(ownerUserID uuid.UUID) bool {
	return a.HasWorkspaceAccess() && (a.CanViewAllCalls() || ownerUserID == a.UserID)
}
