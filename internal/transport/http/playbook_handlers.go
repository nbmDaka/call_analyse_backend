package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"call_analyse_backend/internal/modules/playbooks"
	"call_analyse_backend/internal/modules/workspaces"
	"call_analyse_backend/internal/transport/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

)

func (s server) listPlaybooks(w http.ResponseWriter, r *http.Request) {
	if s.deps.Playbooks == nil {
		writeError(w, r, errors.New("playbook service is not configured"))
		return
	}
	actor, ok := middleware.WorkspaceActorFromContext(r.Context())
	if !ok {
		writeError(w, r, middleware.ErrUnauthenticated)
		return
	}
	list, err := s.deps.Playbooks.List(r.Context(), actor.WorkspaceID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"playbooks": list})
}

func (s server) getPlaybook(w http.ResponseWriter, r *http.Request) {
	if s.deps.Playbooks == nil {
		writeError(w, r, errors.New("playbook service is not configured"))
		return
	}
	actor, ok := middleware.WorkspaceActorFromContext(r.Context())
	if !ok {
		writeError(w, r, middleware.ErrUnauthenticated)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "playbookID"))
	if err != nil {
		writeError(w, r, playbooks.ErrPlaybookNotFound)
		return
	}
	pb, err := s.deps.Playbooks.GetByID(r.Context(), actor.WorkspaceID, id)
	if err != nil {
		if errors.Is(err, playbooks.ErrPlaybookNotFound) {
			writeError(w, r, playbooks.ErrPlaybookNotFound)
		} else {
			writeError(w, r, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"playbook": pb})
}

func (s server) createPlaybook(w http.ResponseWriter, r *http.Request) {
	if s.deps.Playbooks == nil {
		writeError(w, r, errors.New("playbook service is not configured"))
		return
	}
	actor, ok := middleware.WorkspaceActorFromContext(r.Context())
	if !ok {
		writeError(w, r, middleware.ErrUnauthenticated)
		return
	}
	if !actor.CanViewAllCalls() {
		writeError(w, r, workspaces.ErrForbidden)
		return
	}


	var input playbooks.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeInvalid(w, r, "invalid request body")
		return
	}
	created, err := s.deps.Playbooks.Create(r.Context(), actor.WorkspaceID, input)
	if err != nil {
		writeInvalid(w, r, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"playbook": created})
}

func (s server) updatePlaybook(w http.ResponseWriter, r *http.Request) {
	if s.deps.Playbooks == nil {
		writeError(w, r, errors.New("playbook service is not configured"))
		return
	}
	actor, ok := middleware.WorkspaceActorFromContext(r.Context())
	if !ok {
		writeError(w, r, middleware.ErrUnauthenticated)
		return
	}
	if !actor.CanViewAllCalls() {
		writeError(w, r, workspaces.ErrForbidden)
		return
	}


	id, err := uuid.Parse(chi.URLParam(r, "playbookID"))
	if err != nil {
		writeError(w, r, playbooks.ErrPlaybookNotFound)
		return
	}
	var input playbooks.UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeInvalid(w, r, "invalid request body")
		return
	}
	updated, err := s.deps.Playbooks.Update(r.Context(), actor.WorkspaceID, id, input)
	if err != nil {
		if errors.Is(err, playbooks.ErrPlaybookNotFound) {
			writeError(w, r, playbooks.ErrPlaybookNotFound)
		} else {
			writeInvalid(w, r, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"playbook": updated})
}

func (s server) deletePlaybook(w http.ResponseWriter, r *http.Request) {
	if s.deps.Playbooks == nil {
		writeError(w, r, errors.New("playbook service is not configured"))
		return
	}
	actor, ok := middleware.WorkspaceActorFromContext(r.Context())
	if !ok {
		writeError(w, r, middleware.ErrUnauthenticated)
		return
	}
	if !actor.CanViewAllCalls() {
		writeError(w, r, workspaces.ErrForbidden)
		return
	}


	id, err := uuid.Parse(chi.URLParam(r, "playbookID"))
	if err != nil {
		writeError(w, r, playbooks.ErrPlaybookNotFound)
		return
	}
	if err := s.deps.Playbooks.Delete(r.Context(), actor.WorkspaceID, id); err != nil {
		if errors.Is(err, playbooks.ErrPlaybookNotFound) {
			writeError(w, r, playbooks.ErrPlaybookNotFound)
		} else {
			writeError(w, r, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
