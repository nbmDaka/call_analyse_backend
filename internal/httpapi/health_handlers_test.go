package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthLiveAndReadyReportProcessAndDependencyState(t *testing.T) {
	healthy, _ := newTestRouter(t, testDependencies{})
	for _, path := range []string{"/health/live", "/health/ready"} {
		recorder := httptest.NewRecorder()
		healthy.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d: %s", path, recorder.Code, http.StatusOK, recorder.Body.String())
		}
		assertRequestID(t, recorder)
	}

	unready, _ := newTestRouter(t, testDependencies{ready: func(context.Context) error { return errors.New("postgres unavailable") }})
	recorder := httptest.NewRecorder()
	unready.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready failure status = %d, want %d: %s", recorder.Code, http.StatusServiceUnavailable, recorder.Body.String())
	}
	var response map[string]string
	decodeJSON(t, recorder, &response)
	if response["status"] != "not_ready" {
		t.Errorf("ready failure response = %#v, want sanitized not_ready status", response)
	}
}
