package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRequestIDUsesSafeClientValueAndAddsGeneratedID(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if RequestIDFromContext(r.Context()) == "" {
			t.Error("request ID missing from context")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	request.Header.Set("X-Request-ID", "client-request-7")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Header().Get("X-Request-ID") != "client-request-7" {
		t.Errorf("request ID = %q, want safe client value", recorder.Header().Get("X-Request-ID"))
	}

	unsafeRequest := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	unsafeRequest.Header.Set("X-Request-ID", "bad\nheader")
	unsafeRecorder := httptest.NewRecorder()
	handler.ServeHTTP(unsafeRecorder, unsafeRequest)
	if unsafeRecorder.Header().Get("X-Request-ID") == "bad\nheader" || unsafeRecorder.Header().Get("X-Request-ID") == "" {
		t.Errorf("unsafe request ID = %q, want generated safe ID", unsafeRecorder.Header().Get("X-Request-ID"))
	}
}

func TestLoggingAvoidsAuthorizationAndRequestBody(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	handler := RequestID(Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login?token=query-secret", strings.NewReader("password=body-secret"))
	request.Header.Set("Authorization", "Bearer authorization-secret")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	entry := output.String()
	for _, secret := range []string{"authorization-secret", "body-secret", "query-secret"} {
		if strings.Contains(entry, secret) {
			t.Errorf("request log leaks %q: %s", secret, entry)
		}
	}
	if !strings.Contains(entry, "request_id") || !strings.Contains(entry, "status") {
		t.Errorf("request log = %s, want request ID and status fields", entry)
	}
}

func TestWithTimeoutAddsRequestDeadline(t *testing.T) {
	handler := WithTimeout(time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Context().Deadline(); !ok {
			t.Error("request context has no deadline")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if recorder.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}
