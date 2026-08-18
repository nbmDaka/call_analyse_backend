package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"call_analyse_backend/internal/transport/http/middleware"
	"github.com/google/uuid"
)

func (s server) login(w http.ResponseWriter, r *http.Request) {
	if s.deps.Authentication == nil {
		writeError(w, r, errors.New("authentication service is not configured"))
		return
	}
	var input struct{ Email, Password string }
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Email == "" || input.Password == "" {
		writeInvalid(w, r, "email and password are required")
		return
	}
	pair, err := s.deps.Authentication.Login(r.Context(), input.Email, input.Password)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"access_token": pair.AccessToken, "refresh_token": pair.RefreshToken})
}

func (s server) register(w http.ResponseWriter, r *http.Request) {
	if s.deps.Authentication == nil {
		writeError(w, r, errors.New("authentication service is not configured"))
		return
	}
	var input struct{ Email, Password string }
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Email == "" || input.Password == "" {
		writeInvalid(w, r, "email and password are required")
		return
	}
	pair, err := s.deps.Authentication.Register(r.Context(), input.Email, input.Password)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"access_token": pair.AccessToken, "refresh_token": pair.RefreshToken})
}

func (s server) refresh(w http.ResponseWriter, r *http.Request) {
	if s.deps.Authentication == nil {
		writeError(w, r, errors.New("authentication service is not configured"))
		return
	}
	var input struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.RefreshToken == "" {
		writeInvalid(w, r, "refresh token is required")
		return
	}
	pair, err := s.deps.Authentication.Refresh(r.Context(), input.RefreshToken)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"access_token": pair.AccessToken, "refresh_token": pair.RefreshToken})
}

func (s server) logout(w http.ResponseWriter, r *http.Request) {
	if s.deps.Authentication == nil {
		writeError(w, r, errors.New("authentication service is not configured"))
		return
	}
	var input struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.RefreshToken == "" {
		writeInvalid(w, r, "refresh token is required")
		return
	}
	if err := s.deps.Authentication.Logout(r.Context(), input.RefreshToken); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (s server) me(w http.ResponseWriter, r *http.Request) {
	if s.deps.Authentication == nil {
		writeError(w, r, errors.New("authentication service is not configured"))
		return
	}
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		writeError(w, r, middleware.ErrUnauthenticated)
		return
	}
	user, err := s.deps.Authentication.Me(r.Context(), actor.ID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func actorID(r *http.Request) (uuid.UUID, error) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		return uuid.Nil, middleware.ErrUnauthenticated
	}
	return actor.ID, nil
}
