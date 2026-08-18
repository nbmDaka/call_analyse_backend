package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"call_analyse_backend/internal/auth"
	"call_analyse_backend/internal/calls"
	"call_analyse_backend/internal/dashboard"

	"github.com/google/uuid"
)

func TestAuthRoutesReturnTokensAndCurrentUser(t *testing.T) {
	userID := uuid.New()
	authService := &fakeAuthService{
		loginPair:   auth.TokenPair{AccessToken: "access-token", RefreshToken: "refresh-token"},
		refreshPair: auth.TokenPair{AccessToken: "next-access-token", RefreshToken: "next-refresh-token"},
		meUser: auth.PublicUser{
			ID:    userID,
			Email: "manager@example.com",
			Role:  auth.RoleManager,
		},
	}
	handler, tokens := newTestRouter(t, testDependencies{auth: authService})

	t.Run("login", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"manager@example.com","password":"not-logged"}`))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if authService.loginEmail != "manager@example.com" || authService.loginPassword != "not-logged" {
			t.Fatalf("Login() arguments = (%q, %q), want request credentials", authService.loginEmail, authService.loginPassword)
		}
		var response map[string]string
		decodeJSON(t, recorder, &response)
		if response["access_token"] != "access-token" || response["refresh_token"] != "refresh-token" {
			t.Errorf("token response = %#v, want both issued tokens", response)
		}
		assertRequestID(t, recorder)
	})

	t.Run("refresh", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(`{"refresh_token":"old-refresh"}`))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if authService.refreshToken != "old-refresh" {
			t.Errorf("Refresh() token = %q, want request token", authService.refreshToken)
		}
	})

	t.Run("logout", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", strings.NewReader(`{"refresh_token":"old-refresh"}`))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if authService.logoutToken != "old-refresh" {
			t.Errorf("Logout() token = %q, want request token", authService.logoutToken)
		}
	})

	t.Run("me", func(t *testing.T) {
		request := authenticatedRequest(t, tokens, userID, auth.RoleManager, http.MethodGet, "/api/v1/me", nil)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if authService.meID != userID {
			t.Errorf("Me() user ID = %s, want %s", authService.meID, userID)
		}
		var response struct {
			User struct {
				ID    string `json:"id"`
				Email string `json:"email"`
			} `json:"user"`
		}
		decodeJSON(t, recorder, &response)
		if response.User.ID != userID.String() || response.User.Email != "manager@example.com" {
			t.Errorf("me response = %#v, want public user", response.User)
		}
	})
}

func TestProtectedRoutesRejectMissingOrInvalidBearerToken(t *testing.T) {
	handler, _ := newTestRouter(t, testDependencies{})
	for _, authorization := range []string{"", "Bearer not-a-valid-token", "Basic credentials"} {
		t.Run(authorization, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/calls", nil)
			if authorization != "" {
				request.Header.Set("Authorization", authorization)
			}
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			assertAPIError(t, recorder, http.StatusUnauthorized, "UNAUTHENTICATED")
		})
	}
}

func TestUploadAcceptsBoundedAudioAndQueuesProcessing(t *testing.T) {
	managerID := uuid.New()
	callID := uuid.New()
	var gotActor calls.Actor
	var gotAudio string
	var enqueuedID string
	callService := &fakeCallsService{
		createFn: func(_ context.Context, actor calls.Actor, upload calls.Upload) (calls.Call, error) {
			gotActor = actor
			data, err := io.ReadAll(upload.Reader)
			if err != nil {
				return calls.Call{}, err
			}
			gotAudio = string(data)
			return calls.Call{ID: callID, ManagerID: managerID, Status: calls.StatusUploaded, OriginalFilename: upload.Filename, ContentType: upload.ContentType, SizeBytes: upload.Size}, nil
		},
	}
	handler, tokens := newTestRouter(t, testDependencies{
		calls: callService,
		enqueue: func(_ context.Context, id string) error {
			enqueuedID = id
			return nil
		},
	})
	body, contentType := multipartBody(t, "file", "recording.mp3", "audio/mpeg", "audio-data")
	request := authenticatedRequest(t, tokens, managerID, auth.RoleManager, http.MethodPost, "/api/v1/calls", body)
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	if gotActor.ID != managerID || gotActor.Role != auth.RoleManager {
		t.Errorf("Create() actor = %#v, want authenticated manager", gotActor)
	}
	if gotAudio != "audio-data" {
		t.Errorf("Create() audio = %q, want uploaded bytes", gotAudio)
	}
	if enqueuedID != callID.String() {
		t.Errorf("enqueued call ID = %q, want %q", enqueuedID, callID)
	}
	var response struct {
		Call struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"call"`
	}
	decodeJSON(t, recorder, &response)
	if response.Call.ID != callID.String() || response.Call.Status != string(calls.StatusQueued) {
		t.Errorf("upload response = %#v, want queued call", response.Call)
	}
}

func TestUploadRejectsInvalidAudioAndInsufficientRoleBeforeService(t *testing.T) {
	var createCalls int
	callService := &fakeCallsService{createFn: func(context.Context, calls.Actor, calls.Upload) (calls.Call, error) {
		createCalls++
		return calls.Call{}, errors.New("should not create invalid uploads")
	}}
	handler, tokens := newTestRouter(t, testDependencies{calls: callService, maxUploadBytes: 8})
	managerID := uuid.New()

	invalidBody, invalidContentType := multipartBody(t, "file", "recording.txt", "text/plain", "audio")
	invalid := authenticatedRequest(t, tokens, managerID, auth.RoleManager, http.MethodPost, "/api/v1/calls", invalidBody)
	invalid.Header.Set("Content-Type", invalidContentType)
	invalidRecorder := httptest.NewRecorder()
	handler.ServeHTTP(invalidRecorder, invalid)
	assertAPIError(t, invalidRecorder, http.StatusBadRequest, "INVALID_REQUEST")

	tooLargeBody, tooLargeContentType := multipartBody(t, "file", "recording.mp3", "audio/mpeg", "123456789")
	tooLarge := authenticatedRequest(t, tokens, managerID, auth.RoleManager, http.MethodPost, "/api/v1/calls", tooLargeBody)
	tooLarge.Header.Set("Content-Type", tooLargeContentType)
	tooLargeRecorder := httptest.NewRecorder()
	handler.ServeHTTP(tooLargeRecorder, tooLarge)
	assertAPIError(t, tooLargeRecorder, http.StatusBadRequest, "INVALID_REQUEST")

	supervisorBody, supervisorContentType := multipartBody(t, "file", "recording.mp3", "audio/mpeg", "audio")
	supervisor := authenticatedRequest(t, tokens, uuid.New(), auth.RoleSupervisor, http.MethodPost, "/api/v1/calls", supervisorBody)
	supervisor.Header.Set("Content-Type", supervisorContentType)
	supervisorRecorder := httptest.NewRecorder()
	handler.ServeHTTP(supervisorRecorder, supervisor)
	assertAPIError(t, supervisorRecorder, http.StatusForbidden, "FORBIDDEN")

	if createCalls != 0 {
		t.Errorf("Create() calls = %d, want invalid/forbidden uploads rejected before service", createCalls)
	}
}

func TestCallListAndDetailAreScopedByAuthenticatedActor(t *testing.T) {
	managerID := uuid.New()
	callID := uuid.New()
	callService := &fakeCallsService{
		listFn: func(_ context.Context, actor calls.Actor, page calls.Page) (calls.CallPage, error) {
			if actor.ID != managerID || actor.Role != auth.RoleManager {
				return calls.CallPage{}, calls.ErrInvalidActor
			}
			if page.Number != 1 || page.Size != 20 {
				return calls.CallPage{}, errors.New("unexpected pagination")
			}
			return calls.CallPage{Calls: []calls.Call{{ID: callID, ManagerID: managerID, Status: calls.StatusCompleted}}, Total: 1, Page: 1, PerPage: 20, TotalPages: 1}, nil
		},
		detailFn: func(_ context.Context, actor calls.Actor, id uuid.UUID) (calls.Call, error) {
			if actor.ID != managerID || id != callID {
				return calls.Call{}, calls.ErrCallNotFound
			}
			return calls.Call{ID: callID, ManagerID: managerID, Status: calls.StatusCompleted}, nil
		},
	}
	handler, tokens := newTestRouter(t, testDependencies{calls: callService})

	list := authenticatedRequest(t, tokens, managerID, auth.RoleManager, http.MethodGet, "/api/v1/calls", nil)
	listRecorder := httptest.NewRecorder()
	handler.ServeHTTP(listRecorder, list)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d: %s", listRecorder.Code, http.StatusOK, listRecorder.Body.String())
	}

	detail := authenticatedRequest(t, tokens, managerID, auth.RoleManager, http.MethodGet, "/api/v1/calls/"+callID.String(), nil)
	detailRecorder := httptest.NewRecorder()
	handler.ServeHTTP(detailRecorder, detail)
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want %d: %s", detailRecorder.Code, http.StatusOK, detailRecorder.Body.String())
	}

	unowned := authenticatedRequest(t, tokens, uuid.New(), auth.RoleManager, http.MethodGet, "/api/v1/calls/"+callID.String(), nil)
	unownedRecorder := httptest.NewRecorder()
	handler.ServeHTTP(unownedRecorder, unowned)
	assertAPIError(t, unownedRecorder, http.StatusNotFound, "CALL_NOT_FOUND")
}

func TestDashboardSummaryIsScopedByAuthenticatedActor(t *testing.T) {
	managerID := uuid.New()
	dashboardService := &fakeDashboardService{summaryFn: func(_ context.Context, actor calls.Actor) (dashboard.Summary, error) {
		if actor.ID != managerID || actor.Role != auth.RoleManager {
			return dashboard.Summary{}, calls.ErrInvalidActor
		}
		average := 82.5
		return dashboard.Summary{TotalCalls: 2, CompletedCalls: 1, FailedCalls: 0, AverageScore: &average}, nil
	}}
	handler, tokens := newTestRouter(t, testDependencies{dashboard: dashboardService})
	request := authenticatedRequest(t, tokens, managerID, auth.RoleManager, http.MethodGet, "/api/v1/dashboard/summary", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var response struct {
		Summary dashboard.Summary `json:"summary"`
	}
	decodeJSON(t, recorder, &response)
	if response.Summary.TotalCalls != 2 || response.Summary.AverageScore == nil || *response.Summary.AverageScore != 82.5 {
		t.Errorf("dashboard summary = %#v, want scoped aggregate", response.Summary)
	}
}

type fakeAuthService struct {
	loginPair     auth.TokenPair
	loginErr      error
	registerPair  auth.TokenPair
	registerErr   error
	refreshPair   auth.TokenPair
	refreshErr    error
	logoutErr     error
	meUser        auth.PublicUser
	meErr         error
	loginEmail    string
	loginPassword string
	registerEmail string
	registerPassword string
	refreshToken  string
	logoutToken   string
	meID          uuid.UUID
}

func (s *fakeAuthService) Login(_ context.Context, email, password string) (auth.TokenPair, error) {
	s.loginEmail, s.loginPassword = email, password
	return s.loginPair, s.loginErr
}

func (s *fakeAuthService) Register(_ context.Context, email, password string) (auth.TokenPair, error) {
	s.registerEmail, s.registerPassword = email, password
	return s.registerPair, s.registerErr
}

func (s *fakeAuthService) Refresh(_ context.Context, token string) (auth.TokenPair, error) {
	s.refreshToken = token
	return s.refreshPair, s.refreshErr
}

func (s *fakeAuthService) Logout(_ context.Context, token string) error {
	s.logoutToken = token
	return s.logoutErr
}

func (s *fakeAuthService) Me(_ context.Context, id uuid.UUID) (auth.PublicUser, error) {
	s.meID = id
	return s.meUser, s.meErr
}

type fakeCallsService struct {
	createFn func(context.Context, calls.Actor, calls.Upload) (calls.Call, error)
	listFn   func(context.Context, calls.Actor, calls.Page) (calls.CallPage, error)
	detailFn func(context.Context, calls.Actor, uuid.UUID) (calls.Call, error)
}

func (s *fakeCallsService) Create(ctx context.Context, actor calls.Actor, upload calls.Upload) (calls.Call, error) {
	if s.createFn == nil {
		return calls.Call{}, errors.New("unexpected Create call")
	}
	return s.createFn(ctx, actor, upload)
}

func (s *fakeCallsService) List(ctx context.Context, actor calls.Actor, page calls.Page) (calls.CallPage, error) {
	if s.listFn == nil {
		return calls.CallPage{}, errors.New("unexpected List call")
	}
	return s.listFn(ctx, actor, page)
}

func (s *fakeCallsService) Detail(ctx context.Context, actor calls.Actor, id uuid.UUID) (calls.Call, error) {
	if s.detailFn == nil {
		return calls.Call{}, errors.New("unexpected Detail call")
	}
	return s.detailFn(ctx, actor, id)
}

type fakeDashboardService struct {
	summaryFn func(context.Context, calls.Actor) (dashboard.Summary, error)
}

func (s *fakeDashboardService) Summary(ctx context.Context, actor calls.Actor) (dashboard.Summary, error) {
	if s.summaryFn == nil {
		return dashboard.Summary{}, errors.New("unexpected Summary call")
	}
	return s.summaryFn(ctx, actor)
}

type testDependencies struct {
	auth           *fakeAuthService
	calls          *fakeCallsService
	dashboard      *fakeDashboardService
	enqueue        func(context.Context, string) error
	ready          func(context.Context) error
	maxUploadBytes int64
}

func newTestRouter(t *testing.T, overrides testDependencies) (http.Handler, auth.TokenManager) {
	t.Helper()
	tokens, err := auth.NewTokenManager("test-access-secret", "test-refresh-secret", time.Minute, time.Hour)
	if err != nil {
		t.Fatalf("NewTokenManager() error = %v", err)
	}
	if overrides.auth == nil {
		overrides.auth = &fakeAuthService{}
	}
	if overrides.calls == nil {
		overrides.calls = &fakeCallsService{
			listFn: func(context.Context, calls.Actor, calls.Page) (calls.CallPage, error) { return calls.CallPage{}, nil },
			detailFn: func(context.Context, calls.Actor, uuid.UUID) (calls.Call, error) {
				return calls.Call{}, calls.ErrCallNotFound
			},
		}
	}
	if overrides.dashboard == nil {
		overrides.dashboard = &fakeDashboardService{summaryFn: func(context.Context, calls.Actor) (dashboard.Summary, error) { return dashboard.Summary{}, nil }}
	}
	if overrides.enqueue == nil {
		overrides.enqueue = func(context.Context, string) error { return nil }
	}
	if overrides.ready == nil {
		overrides.ready = func(context.Context) error { return nil }
	}
	if overrides.maxUploadBytes == 0 {
		overrides.maxUploadBytes = 1024
	}
	return NewRouter(Dependencies{
		Authentication: overrides.auth,
		Calls:          overrides.calls,
		Dashboard:      overrides.dashboard,
		Tokens:         tokens,
		EnqueueCall:    overrides.enqueue,
		Ready:          overrides.ready,
		MaxUploadBytes: overrides.maxUploadBytes,
		RequestTimeout: time.Second,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}), tokens
}

func authenticatedRequest(t *testing.T, tokens auth.TokenManager, userID uuid.UUID, role auth.Role, method, path string, body io.Reader) *http.Request {
	t.Helper()
	token, err := tokens.IssueAccess(auth.User{ID: userID, Role: role})
	if err != nil {
		t.Fatalf("IssueAccess() error = %v", err)
	}
	request := httptest.NewRequest(method, path, body)
	request.Header.Set("Authorization", "Bearer "+token)
	return request
}

func multipartBody(t *testing.T, field, filename, contentType, contents string) (*bytes.Reader, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": []string{`form-data; name="` + field + `"; filename="` + filename + `"`},
		"Content-Type":        []string{contentType},
	})
	if err != nil {
		t.Fatalf("CreatePart() error = %v", err)
	}
	if _, err := io.WriteString(part, contents); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return bytes.NewReader(body.Bytes()), writer.FormDataContentType()
}

func decodeJSON(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.NewDecoder(recorder.Body).Decode(target); err != nil {
		t.Fatalf("decode JSON response %q: %v", recorder.Body.String(), err)
	}
}

func assertRequestID(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if recorder.Header().Get("X-Request-ID") == "" {
		t.Error("X-Request-ID header is empty")
	}
}

func assertAPIError(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if recorder.Code != wantStatus {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, wantStatus, recorder.Body.String())
	}
	var response struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
		RequestID string `json:"request_id"`
	}
	decodeJSON(t, recorder, &response)
	if response.Error.Code != wantCode {
		t.Errorf("error code = %q, want %q", response.Error.Code, wantCode)
	}
	if response.RequestID == "" || response.RequestID != recorder.Header().Get("X-Request-ID") {
		t.Errorf("response request ID = %q, header = %q; want matching non-empty IDs", response.RequestID, recorder.Header().Get("X-Request-ID"))
	}
}
