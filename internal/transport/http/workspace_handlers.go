package httpapi

import (
	"encoding/json"
	"net/http"

	"call_analyse_backend/internal/modules/memberships"
	"call_analyse_backend/internal/modules/workspaces"
	"call_analyse_backend/internal/transport/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s server) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		writeError(w, r, middleware.ErrUnauthenticated)
		return
	}
	items, err := s.deps.Workspaces.List(r.Context(), actor.ID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": items})
}

func (s server) createWorkspace(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		writeError(w, r, middleware.ErrUnauthenticated)
		return
	}
	var input struct {
		Name string          `json:"name"`
		Type workspaces.Type `json:"type"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || input.Type != workspaces.TypeCompany {
		writeInvalid(w, r, "company workspace name and type are required")
		return
	}
	item, err := s.deps.Workspaces.CreateCompany(r.Context(), actor.ID, input.Name)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"workspace": item})
}

func (s server) getWorkspace(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		writeError(w, r, middleware.ErrUnauthenticated)
		return
	}
	workspaceID, err := uuid.Parse(chi.URLParam(r, "workspaceID"))
	if err != nil {
		writeError(w, r, workspaces.ErrWorkspaceNotFound)
		return
	}
	item, err := s.deps.Workspaces.Get(r.Context(), actor.ID, workspaceID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspace": item})
}

func (s server) renameWorkspace(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.WorkspaceActorFromContext(r.Context())
	if !ok {
		writeError(w, r, workspaces.ErrForbidden)
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		writeInvalid(w, r, "workspace name is required")
		return
	}
	item, err := s.deps.Workspaces.Rename(r.Context(), actor, input.Name)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspace": item})
}

func (s server) listMembers(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.WorkspaceActorFromContext(r.Context())
	if !ok {
		writeError(w, r, workspaces.ErrForbidden)
		return
	}
	items, err := s.deps.Memberships.List(r.Context(), actor)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": items})
}

func (s server) createMember(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.WorkspaceActorFromContext(r.Context())
	if !ok {
		writeError(w, r, workspaces.ErrForbidden)
		return
	}
	var input struct {
		Email string          `json:"email"`
		Role  workspaces.Role `json:"role"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || input.Email == "" {
		writeInvalid(w, r, "email and workspace role are required")
		return
	}
	item, err := s.deps.Memberships.Create(r.Context(), actor, memberships.CreateInput{Email: input.Email, Role: input.Role})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"member": item})
}

func (s server) updateMember(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.WorkspaceActorFromContext(r.Context())
	if !ok {
		writeError(w, r, workspaces.ErrForbidden)
		return
	}
	membershipID, err := uuid.Parse(chi.URLParam(r, "membershipID"))
	if err != nil {
		writeError(w, r, workspaces.ErrMembershipNotFound)
		return
	}
	var input struct {
		Role                   *workspaces.Role             `json:"role"`
		Status                 *workspaces.MembershipStatus `json:"status"`
		SupervisorMembershipID *uuid.UUID                   `json:"supervisor_membership_id"`
		ClearSupervisor        bool                         `json:"clear_supervisor"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		writeInvalid(w, r, "invalid membership update")
		return
	}
	item, err := s.deps.Memberships.Update(r.Context(), actor, membershipID, memberships.UpdateInput{Role: input.Role, Status: input.Status, SupervisorMembershipID: input.SupervisorMembershipID, ClearSupervisor: input.ClearSupervisor})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"member": item})
}

func (s server) deleteMember(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.WorkspaceActorFromContext(r.Context())
	if !ok {
		writeError(w, r, workspaces.ErrForbidden)
		return
	}
	membershipID, err := uuid.Parse(chi.URLParam(r, "membershipID"))
	if err != nil {
		writeError(w, r, workspaces.ErrMembershipNotFound)
		return
	}
	if err := s.deps.Memberships.Delete(r.Context(), actor, membershipID); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
