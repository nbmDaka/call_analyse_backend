package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"call_analyse_backend/internal/modules/auth"
	"call_analyse_backend/internal/modules/workspaces"

	"github.com/google/uuid"
)

type actorResolverFunc func(context.Context, uuid.UUID, workspaces.PlatformRole, uuid.UUID) (workspaces.Actor, error)

func (f actorResolverFunc) ResolveActor(ctx context.Context, userID uuid.UUID, role workspaces.PlatformRole, workspaceID uuid.UUID) (workspaces.Actor, error) {
	return f(ctx, userID, role, workspaceID)
}

func TestCrossTenantWorkspaceRequestReturns404BeforeService(t *testing.T) {
	tokens, err := auth.NewTokenManager("access-secret", "refresh-secret", time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	userID, workspaceID := uuid.New(), uuid.New()
	router := NewRouter(Dependencies{Tokens: tokens, Calls: &fakeCallsService{}, WorkspaceActors: actorResolverFunc(func(context.Context, uuid.UUID, workspaces.PlatformRole, uuid.UUID) (workspaces.Actor, error) {
		return workspaces.Actor{}, workspaces.ErrWorkspaceNotFound
	})})
	request := authenticatedRequest(t, tokens, userID, auth.RoleManager, http.MethodGet, "/api/v1/workspaces/"+workspaceID.String()+"/calls", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", response.Code, response.Body.String())
	}
}

func TestDisabledMembershipLosesAccessImmediately(t *testing.T) {
	tokens, err := auth.NewTokenManager("access-secret", "refresh-secret", time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	workspaceID := uuid.New()
	router := NewRouter(Dependencies{Tokens: tokens, Calls: &fakeCallsService{}, WorkspaceActors: actorResolverFunc(func(context.Context, uuid.UUID, workspaces.PlatformRole, uuid.UUID) (workspaces.Actor, error) {
		return workspaces.Actor{}, workspaces.ErrMembershipDisabled
	})})
	request := authenticatedRequest(t, tokens, uuid.New(), auth.RoleManager, http.MethodGet, "/api/v1/workspaces/"+workspaceID.String()+"/calls", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
}

func TestSuspendedWorkspaceRejectsUpload(t *testing.T) {
	tokens, err := auth.NewTokenManager("access-secret", "refresh-secret", time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	userID, workspaceID := uuid.New(), uuid.New()
	router := NewRouter(Dependencies{Tokens: tokens, Calls: &fakeCallsService{}, WorkspaceActors: actorResolverFunc(func(context.Context, uuid.UUID, workspaces.PlatformRole, uuid.UUID) (workspaces.Actor, error) {
		return workspaces.Actor{UserID: userID, WorkspaceID: workspaceID, MembershipID: uuid.New(), WorkspaceRole: workspaces.RoleManager, MembershipStatus: workspaces.MembershipActive, WorkspaceStatus: workspaces.StatusSuspended}, nil
	})})
	request := authenticatedRequest(t, tokens, userID, auth.RoleManager, http.MethodPost, "/api/v1/workspaces/"+workspaceID.String()+"/calls", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
}
