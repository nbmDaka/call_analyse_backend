package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"call_analyse_backend/internal/modules/auth"
	"call_analyse_backend/internal/modules/calls"

	"github.com/google/uuid"
)

func TestErrorResponseUsesRequestIDAndSanitizesUnexpectedFailures(t *testing.T) {
	handler, tokens := newTestRouter(t, testDependencies{calls: &fakeCallsService{
		listFn: func(_ context.Context, _ calls.Actor, _ calls.Page) (calls.CallPage, error) {
			return calls.CallPage{}, errors.New("database password=never-return-this")
		},
	}})
	request := authenticatedRequest(t, tokens, uuid.New(), auth.RoleManager, http.MethodGet, "/api/v1/calls", nil)
	request.Header.Set("X-Request-ID", "client-request-42")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	assertAPIError(t, recorder, http.StatusInternalServerError, "INTERNAL_ERROR")
	if recorder.Header().Get("X-Request-ID") != "client-request-42" {
		t.Errorf("request ID = %q, want client request ID", recorder.Header().Get("X-Request-ID"))
	}
	if strings.Contains(recorder.Body.String(), "password") || strings.Contains(recorder.Body.String(), "never-return-this") {
		t.Errorf("error response leaks internal details: %s", recorder.Body.String())
	}
}

func TestMiddlewareReturnsJSONForUnknownMethodAndPreservesValidRequestID(t *testing.T) {
	handler, _ := newTestRouter(t, testDependencies{})
	request := httptest.NewRequest(http.MethodDelete, "/health/live", nil)
	request.Header.Set("X-Request-ID", "safe-request-id")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	assertAPIError(t, recorder, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
	if recorder.Header().Get("X-Request-ID") != "safe-request-id" {
		t.Errorf("request ID = %q, want safe-request-id", recorder.Header().Get("X-Request-ID"))
	}
}
