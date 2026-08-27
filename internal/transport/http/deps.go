package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"call_analyse_backend/internal/modules/auth"
	"call_analyse_backend/internal/modules/calls"
	"call_analyse_backend/internal/modules/dashboard"
	"call_analyse_backend/internal/modules/golden_standards"
	"call_analyse_backend/internal/modules/invitations"
	"call_analyse_backend/internal/modules/memberships"
	platformadmin "call_analyse_backend/internal/modules/platform"
	"call_analyse_backend/internal/modules/playbooks"
	"call_analyse_backend/internal/modules/workspaces"

	"github.com/google/uuid"

)

type authService interface {
	Login(context.Context, string, string) (auth.TokenPair, error)
	Register(context.Context, string, string) error
	ConfirmEmail(context.Context, string) error
	ResendVerification(context.Context, string) error
	RequestPasswordReset(context.Context, string) error
	ResetPassword(context.Context, string, string) error
	Refresh(context.Context, string) (auth.TokenPair, error)
	Logout(context.Context, string) error
	Me(context.Context, uuid.UUID) (auth.PublicUser, error)
}

type callsService interface {
	Create(context.Context, calls.Actor, calls.Upload) (calls.Call, error)
	List(context.Context, calls.Actor, calls.Page) (calls.CallPage, error)
	Detail(context.Context, calls.Actor, uuid.UUID) (calls.Call, error)
}

type dashboardService interface {
	Summary(context.Context, calls.Actor) (dashboard.Summary, error)
}

type workspaceService interface {
	List(context.Context, uuid.UUID) ([]workspaces.AvailableWorkspace, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (workspaces.AvailableWorkspace, error)
	CreateCompany(context.Context, uuid.UUID, string) (workspaces.AvailableWorkspace, error)
	Rename(context.Context, workspaces.Actor, string) (workspaces.Workspace, error)
}

type membershipService interface {
	List(context.Context, workspaces.Actor) ([]workspaces.Membership, error)
	Create(context.Context, workspaces.Actor, memberships.CreateInput) (workspaces.Membership, error)
	Update(context.Context, workspaces.Actor, uuid.UUID, memberships.UpdateInput) (workspaces.Membership, error)
	Delete(context.Context, workspaces.Actor, uuid.UUID) error
}

type platformService interface {
	CreateCompany(context.Context, uuid.UUID, workspaces.PlatformRole, uuid.UUID, string) (workspaces.Workspace, error)
	ListWorkspaces(context.Context, workspaces.PlatformRole, *workspaces.Type) ([]workspaces.Workspace, error)
	ListUsers(context.Context, workspaces.PlatformRole) ([]platformadmin.User, error)
	SetWorkspaceStatus(context.Context, uuid.UUID, workspaces.PlatformRole, uuid.UUID, workspaces.Status) (workspaces.Workspace, error)
	SetUserStatus(context.Context, uuid.UUID, workspaces.PlatformRole, uuid.UUID, string) (platformadmin.User, error)
	SystemMetrics(context.Context, workspaces.PlatformRole) (platformadmin.Metrics, error)
}

type playbookService interface {
	List(context.Context, uuid.UUID) ([]playbooks.Playbook, error)
	GetByID(context.Context, uuid.UUID, uuid.UUID) (playbooks.Playbook, error)
	GetDefault(context.Context, uuid.UUID) (playbooks.Playbook, error)
	Create(context.Context, uuid.UUID, playbooks.CreateInput) (playbooks.Playbook, error)
	Update(context.Context, uuid.UUID, uuid.UUID, playbooks.UpdateInput) (playbooks.Playbook, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
}

type goldenStandardsService interface {
	List(context.Context, uuid.UUID, string) ([]golden_standards.GoldenStandard, error)
	GetByID(context.Context, uuid.UUID, uuid.UUID) (golden_standards.GoldenStandard, error)
	Create(context.Context, uuid.UUID, golden_standards.CreateInput) (golden_standards.GoldenStandard, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
}

type invitationsService interface {
	Invite(context.Context, workspaces.Actor, invitations.CreateInput) (invitations.Invitation, error)
	ListPending(context.Context, workspaces.Actor) ([]invitations.Invitation, error)
	Revoke(context.Context, workspaces.Actor, uuid.UUID) error
	GetInfo(context.Context, string) (invitations.InvitationInfo, error)
	Accept(context.Context, uuid.UUID, string) (uuid.UUID, error)
	RegisterAndAccept(context.Context, string, string) (auth.TokenPair, auth.PublicUser, uuid.UUID, error)
}

// Dependencies are the application boundaries required by the HTTP layer.
type Dependencies struct {
	Authentication       authService
	CORSAllowedOrigins   []string
	Calls                callsService
	Dashboard            dashboardService
	Workspaces           workspaceService
	WorkspaceActors      workspaces.ActorResolver
	Memberships          membershipService
	Invitations          invitationsService
	Platform             platformService
	Playbooks            playbookService
	GoldenStandards      goldenStandardsService
	Tokens               auth.TokenManager
	EnqueueCall          func(context.Context, string) error
	EnqueueWorkspaceCall func(context.Context, string, string) error
	Ready                func(context.Context) error
	MaxUploadBytes       int64
	RequestTimeout       time.Duration
	Logger               *slog.Logger
}


type server struct {
	deps Dependencies
}

var _ http.Handler = http.HandlerFunc(nil)
