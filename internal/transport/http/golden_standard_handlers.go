package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"call_analyse_backend/internal/modules/golden_standards"
	"call_analyse_backend/internal/modules/workspaces"
	"call_analyse_backend/internal/transport/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s server) listGoldenStandards(w http.ResponseWriter, r *http.Request) {
	if s.deps.GoldenStandards == nil {
		writeError(w, r, errors.New("golden standards service is not configured"))
		return
	}
	actor, ok := middleware.WorkspaceActorFromContext(r.Context())
	if !ok {
		writeError(w, r, middleware.ErrUnauthenticated)
		return
	}
	category := r.URL.Query().Get("category")
	list, err := s.deps.GoldenStandards.List(r.Context(), actor.WorkspaceID, category)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"golden_standards": list})
}

func (s server) getGoldenStandard(w http.ResponseWriter, r *http.Request) {
	if s.deps.GoldenStandards == nil {
		writeError(w, r, errors.New("golden standards service is not configured"))
		return
	}
	actor, ok := middleware.WorkspaceActorFromContext(r.Context())
	if !ok {
		writeError(w, r, middleware.ErrUnauthenticated)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, golden_standards.ErrGoldenStandardNotFound)
		return
	}
	item, err := s.deps.GoldenStandards.GetByID(r.Context(), actor.WorkspaceID, id)
	if err != nil {
		if errors.Is(err, golden_standards.ErrGoldenStandardNotFound) {
			writeError(w, r, golden_standards.ErrGoldenStandardNotFound)
		} else {
			writeError(w, r, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"golden_standard": item})
}

func (s server) createGoldenStandard(w http.ResponseWriter, r *http.Request) {
	if s.deps.GoldenStandards == nil {
		writeError(w, r, errors.New("golden standards service is not configured"))
		return
	}
	actor, ok := middleware.WorkspaceActorFromContext(r.Context())
	if !ok {
		writeError(w, r, middleware.ErrUnauthenticated)
		return
	}
	if !actor.CanViewAllCalls() && actor.WorkspaceRole != workspaces.RoleSupervisor {
		writeError(w, r, workspaces.ErrForbidden)
		return
	}

	var input golden_standards.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeInvalid(w, r, "invalid request body")
		return
	}
	created, err := s.deps.GoldenStandards.Create(r.Context(), actor.WorkspaceID, input)
	if err != nil {
		writeInvalid(w, r, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"golden_standard": created})
}

func (s server) deleteGoldenStandard(w http.ResponseWriter, r *http.Request) {
	if s.deps.GoldenStandards == nil {
		writeError(w, r, errors.New("golden standards service is not configured"))
		return
	}
	actor, ok := middleware.WorkspaceActorFromContext(r.Context())
	if !ok {
		writeError(w, r, middleware.ErrUnauthenticated)
		return
	}
	if !actor.CanViewAllCalls() && actor.WorkspaceRole != workspaces.RoleSupervisor {
		writeError(w, r, workspaces.ErrForbidden)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, golden_standards.ErrGoldenStandardNotFound)
		return
	}
	if err := s.deps.GoldenStandards.Delete(r.Context(), actor.WorkspaceID, id); err != nil {
		if errors.Is(err, golden_standards.ErrGoldenStandardNotFound) {
			writeError(w, r, golden_standards.ErrGoldenStandardNotFound)
		} else {
			writeError(w, r, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
