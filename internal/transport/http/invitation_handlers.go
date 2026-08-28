package httpapi

import (
	"encoding/json"
	"net/http"

	"call_analyse_backend/internal/modules/invitations"
	"call_analyse_backend/internal/transport/http/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s server) createInvitation(w http.ResponseWriter, r *http.Request) {
	if s.deps.Invitations == nil {
		writeInvalid(w, r, "invitations service unavailable")
		return
	}
	actor, ok := middleware.WorkspaceActorFromContext(r.Context())
	if !ok {
		writeError(w, r, middleware.ErrUnauthenticated)
		return
	}
	var input invitations.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeInvalid(w, r, "invalid request body")
		return
	}
	invitation, err := s.deps.Invitations.Invite(r.Context(), actor, input)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"invitation": invitation})
}

func (s server) listInvitations(w http.ResponseWriter, r *http.Request) {
	if s.deps.Invitations == nil {
		writeInvalid(w, r, "invitations service unavailable")
		return
	}
	actor, ok := middleware.WorkspaceActorFromContext(r.Context())
	if !ok {
		writeError(w, r, middleware.ErrUnauthenticated)
		return
	}
	list, err := s.deps.Invitations.ListPending(r.Context(), actor)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"invitations": list})
}

func (s server) revokeInvitation(w http.ResponseWriter, r *http.Request) {
	if s.deps.Invitations == nil {
		writeInvalid(w, r, "invitations service unavailable")
		return
	}
	actor, ok := middleware.WorkspaceActorFromContext(r.Context())
	if !ok {
		writeError(w, r, middleware.ErrUnauthenticated)
		return
	}
	invitationID, err := uuid.Parse(chi.URLParam(r, "invitationID"))
	if err != nil {
		writeInvalid(w, r, "invalid invitation id")
		return
	}
	if err := s.deps.Invitations.Revoke(r.Context(), actor, invitationID); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "revoked"})
}

func (s server) getInvitation(w http.ResponseWriter, r *http.Request) {
	if s.deps.Invitations == nil {
		writeInvalid(w, r, "invitations service unavailable")
		return
	}
	token := chi.URLParam(r, "token")
	if token == "" {
		writeInvalid(w, r, "token is required")
		return
	}
	info, err := s.deps.Invitations.GetInfo(r.Context(), token)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"invitation": info})
}

func (s server) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	if s.deps.Invitations == nil {
		writeInvalid(w, r, "invitations service unavailable")
		return
	}
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		writeError(w, r, middleware.ErrUnauthenticated)
		return
	}
	token := chi.URLParam(r, "token")
	if token == "" {
		writeInvalid(w, r, "token is required")
		return
	}
	workspaceID, err := s.deps.Invitations.Accept(r.Context(), actor.ID, token)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "accepted", "workspace_id": workspaceID})
}

func (s server) registerByInvitation(w http.ResponseWriter, r *http.Request) {
	if s.deps.Invitations == nil {
		writeInvalid(w, r, "invitations service unavailable")
		return
	}
	token := chi.URLParam(r, "token")
	if token == "" {
		writeInvalid(w, r, "token is required")
		return
	}
	var input invitations.RegisterByInviteInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeInvalid(w, r, "invalid request body")
		return
	}
	tokens, user, workspaceID, err := s.deps.Invitations.RegisterAndAccept(r.Context(), token, input.Password)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
		"user":          user,
		"workspace_id":  workspaceID,
	})
}
