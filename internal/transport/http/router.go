package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"call_analyse_backend/internal/modules/auth"
	"call_analyse_backend/internal/modules/workspaces"
	"call_analyse_backend/internal/transport/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func NewRouter(deps Dependencies) http.Handler {
	if deps.RequestTimeout <= 0 {
		deps.RequestTimeout = 30 * time.Second
	}
	s := server{deps: deps}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logging(deps.Logger))
	r.Use(middleware.WithTimeout(deps.RequestTimeout))
	r.Use(cors(deps.CORSAllowedOrigins))
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		writeJSONError(w, req, http.StatusNotFound, "NOT_FOUND", "resource not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		writeJSONError(w, req, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	})
	r.Get("/health/live", healthLive)
	r.Get("/health/ready", s.healthReady)
	r.Route("/api/v1", func(api chi.Router) {
		api.Post("/auth/login", s.login)
		api.Post("/auth/register", s.register)
		api.Get("/auth/verify-email", s.verifyEmail)
		api.Post("/auth/resend-verification", s.resendVerification)
		api.Post("/auth/forgot-password", s.forgotPassword)
		api.Post("/auth/reset-password", s.resetPassword)
		api.Post("/auth/refresh", s.refresh)
		api.Post("/auth/logout", s.logout)
		api.Get("/invitations/{token}", s.getInvitation)
		api.Post("/invitations/{token}/register", s.registerByInvitation)
		api.Group(func(protected chi.Router) {
			protected.Use(s.authenticate)
			protected.Get("/me", s.me)
			protected.Post("/invitations/{token}/accept", s.acceptInvitation)
			protected.Get("/workspaces", s.listWorkspaces)
			protected.Post("/workspaces", s.createWorkspace)
			protected.Route("/workspaces/{workspaceID}", func(workspace chi.Router) {
				workspace.Use(s.resolveWorkspaceActor)
				workspace.Get("/", s.getWorkspace)
				workspace.Patch("/", s.renameWorkspace)
				workspace.Delete("/", s.deleteWorkspace)
				workspace.Get("/members", s.listMembers)
				workspace.Post("/members", s.createMember)
				workspace.Patch("/members/{membershipID}", s.updateMember)
				workspace.Delete("/members/{membershipID}", s.deleteMember)
				workspace.Get("/invitations", s.listInvitations)
				workspace.Post("/invitations", s.createInvitation)
				workspace.Delete("/invitations/{invitationID}", s.revokeInvitation)
				workspace.Get("/calls", s.listCalls)

				workspace.Post("/calls", s.createCall)
				workspace.Get("/calls/{id}", s.detailCall)
				workspace.Get("/dashboard", s.dashboardSummary)
				workspace.Get("/playbooks", s.listPlaybooks)
				workspace.Post("/playbooks", s.createPlaybook)
				workspace.Get("/playbooks/{playbookID}", s.getPlaybook)
				workspace.Patch("/playbooks/{playbookID}", s.updatePlaybook)
				workspace.Delete("/playbooks/{playbookID}", s.deletePlaybook)
				workspace.Get("/golden-standards", s.listGoldenStandards)
				workspace.Post("/golden-standards", s.createGoldenStandard)
				workspace.Get("/golden-standards/{id}", s.getGoldenStandard)
				workspace.Delete("/golden-standards/{id}", s.deleteGoldenStandard)
			})

			protected.Get("/platform/workspaces", s.platformWorkspaces)
			protected.Post("/platform/workspaces", s.platformCreateWorkspace)
			protected.Get("/platform/users", s.platformUsers)
			protected.Get("/platform/calls", s.platformCalls)
			protected.Get("/platform/metrics", s.platformMetrics)
			protected.Patch("/platform/workspaces/{workspaceID}/status", s.platformWorkspaceStatus)
			protected.Patch("/platform/users/{userID}/status", s.platformUserStatus)
			protected.Patch("/platform/users/{userID}/role", s.platformUserRole)
			// Deprecated compatibility routes. New clients must use explicit workspace routes.
			protected.Post("/calls", s.createCall)
			protected.Get("/calls", s.listCalls)
			protected.Get("/calls/{id}", s.detailCall)
			protected.Get("/dashboard/summary", s.dashboardSummary)
		})
	})
	return r
}

func (s server) resolveWorkspaceActor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := middleware.ActorFromContext(r.Context())
		if !ok || s.deps.WorkspaceActors == nil {
			writeError(w, r, middleware.ErrUnauthenticated)
			return
		}
		workspaceID, err := uuid.Parse(chi.URLParam(r, "workspaceID"))
		if err != nil {
			writeError(w, r, workspaces.ErrWorkspaceNotFound)
			return
		}
		actor, err := s.deps.WorkspaceActors.ResolveActor(r.Context(), identity.ID, identity.PlatformRole, workspaceID)
		if err != nil {
			writeError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(middleware.WithWorkspaceActor(r.Context(), actor)))
	})
}

func (s server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.deps.Tokens == nil {
			writeError(w, r, middleware.ErrUnauthenticated)
			return
		}
		actor, err := middleware.ParseActor(r, s.deps.Tokens)
		if err != nil {
			writeError(w, r, err)
			return
		}
		if s.deps.Authentication != nil {
			user, err := s.deps.Authentication.Me(r.Context(), actor.ID)
			if err != nil {
				writeError(w, r, middleware.ErrUnauthenticated)
				return
			}
			if user.Status == "suspended" {
				writeError(w, r, auth.ErrUserSuspended)
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(middleware.WithActor(r.Context(), actor)))
	})
}

type callsNotFoundError struct{}

func (callsNotFoundError) Error() string { return "not found" }
func writeJSONError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}, "request_id": middleware.RequestIDFromContext(r.Context())})
}
