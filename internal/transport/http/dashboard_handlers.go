package httpapi

import (
	"call_analyse_backend/internal/transport/http/middleware"
	"errors"
	"net/http"
)

func (s server) dashboardSummary(w http.ResponseWriter, r *http.Request) {
	if s.deps.Dashboard == nil {
		writeError(w, r, errors.New("dashboard service is not configured"))
		return
	}
	actor, _, ok := callActorFromRequest(r)
	if !ok {
		writeError(w, r, middleware.ErrUnauthenticated)
		return
	}
	summary, err := s.deps.Dashboard.Summary(r.Context(), actor)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"summary": summary})
}
