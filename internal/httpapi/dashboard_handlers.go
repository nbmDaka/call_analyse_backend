package httpapi

import (
	"call_analyse_backend/internal/calls"
	"call_analyse_backend/internal/middleware"
	"errors"
	"net/http"
)

func (s server) dashboardSummary(w http.ResponseWriter, r *http.Request) {
	if s.deps.Dashboard == nil {
		writeError(w, r, errors.New("dashboard service is not configured"))
		return
	}
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		writeError(w, r, middleware.ErrUnauthenticated)
		return
	}
	summary, err := s.deps.Dashboard.Summary(r.Context(), calls.Actor{ID: actor.ID, Role: actor.Role})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"summary": summary})
}
