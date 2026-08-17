package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"call_analyse_backend/internal/middleware"

	"github.com/go-chi/chi/v5"
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
		api.Post("/auth/refresh", s.refresh)
		api.Post("/auth/logout", s.logout)
		api.Group(func(protected chi.Router) {
			protected.Use(s.authenticate)
			protected.Get("/me", s.me)
			protected.Post("/calls", s.createCall)
			protected.Get("/calls", s.listCalls)
			protected.Get("/calls/{id}", s.detailCall)
			protected.Get("/dashboard/summary", s.dashboardSummary)
		})
	})
	return r
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
