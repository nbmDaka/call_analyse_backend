package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"call_analyse_backend/internal/modules/platform"
	"call_analyse_backend/internal/modules/workspaces"
	"call_analyse_backend/internal/transport/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s server) platformWorkspaces(w http.ResponseWriter, r *http.Request) {
	actor, _ := middleware.ActorFromContext(r.Context())
	var filter *workspaces.Type
	if value := r.URL.Query().Get("type"); value != "" {
		parsed := workspaces.Type(value)
		filter = &parsed
	}
	items, err := s.deps.Platform.ListWorkspaces(r.Context(), actor.PlatformRole, filter)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": items})
}

func (s server) platformCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	actor, _ := middleware.ActorFromContext(r.Context())
	var input struct {
		Name        string    `json:"name"`
		OwnerUserID uuid.UUID `json:"owner_user_id"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || input.Name == "" || input.OwnerUserID == uuid.Nil {
		writeInvalid(w, r, "name and owner_user_id are required")
		return
	}
	item, err := s.deps.Platform.CreateCompany(r.Context(), actor.ID, actor.PlatformRole, input.OwnerUserID, input.Name)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"workspace": item})
}

func (s server) platformUsers(w http.ResponseWriter, r *http.Request) {
	actor, _ := middleware.ActorFromContext(r.Context())
	items, err := s.deps.Platform.ListUsers(r.Context(), actor.PlatformRole)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": items})
}

func (s server) platformMetrics(w http.ResponseWriter, r *http.Request) {
	actor, _ := middleware.ActorFromContext(r.Context())
	item, err := s.deps.Platform.SystemMetrics(r.Context(), actor.PlatformRole)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"metrics": item})
}

func (s server) platformWorkspaceStatus(w http.ResponseWriter, r *http.Request) {
	actor, _ := middleware.ActorFromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "workspaceID"))
	if err != nil {
		writeError(w, r, workspaces.ErrWorkspaceNotFound)
		return
	}
	var input struct {
		Status workspaces.Status `json:"status"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		writeInvalid(w, r, "status is required")
		return
	}
	item, err := s.deps.Platform.SetWorkspaceStatus(r.Context(), actor.ID, actor.PlatformRole, id, input.Status)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspace": item})
}

func (s server) platformUserStatus(w http.ResponseWriter, r *http.Request) {
	actor, _ := middleware.ActorFromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		writeInvalid(w, r, "valid user ID is required")
		return
	}
	var input struct {
		Status string `json:"status"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		writeInvalid(w, r, "status is required")
		return
	}
	item, err := s.deps.Platform.SetUserStatus(r.Context(), actor.ID, actor.PlatformRole, id, input.Status)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": item})
}

func (s server) platformUserRole(w http.ResponseWriter, r *http.Request) {
	actor, _ := middleware.ActorFromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		writeInvalid(w, r, "valid user ID is required")
		return
	}
	var input struct {
		PlatformRole workspaces.PlatformRole `json:"platform_role"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || (input.PlatformRole != workspaces.PlatformRoleUser && input.PlatformRole != workspaces.PlatformRoleSuperAdmin) {
		writeInvalid(w, r, "valid platform_role ('user' or 'super_admin') is required")
		return
	}
	item, err := s.deps.Platform.SetUserPlatformRole(r.Context(), actor.ID, actor.PlatformRole, id, input.PlatformRole)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": item})
}

func (s server) platformCalls(w http.ResponseWriter, r *http.Request) {
	actor, _ := middleware.ActorFromContext(r.Context())
	filter := platform.CallListFilter{
		Limit:  50,
		Offset: 0,
	}
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = &status
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}
	page, err := s.deps.Platform.ListCalls(r.Context(), actor.PlatformRole, filter)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}
