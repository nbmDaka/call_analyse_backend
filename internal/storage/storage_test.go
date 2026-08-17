package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type contractStore struct{}

func (contractStore) Put(context.Context, string, io.Reader, int64, string) error { return nil }
func (contractStore) Get(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (contractStore) Delete(context.Context, string) error { return nil }

func TestObjectStoreContractSupportsStreamingOperations(t *testing.T) {
	var store ObjectStore = contractStore{}
	if err := store.Put(context.Background(), "calls/id/object.mp3", strings.NewReader("audio"), 5, "audio/mpeg"); err != nil {
		t.Fatalf("Put() error = %v, want nil", err)
	}
}

func TestNewMinIOStoreEnsuresMissingBucketBeforeReturning(t *testing.T) {
	var located, checked, created bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Trim(r.URL.Path, "/") != "call-audio" {
			t.Errorf("request path = %q, want configured bucket", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodGet:
			if !r.URL.Query().Has("location") {
				t.Errorf("GET query = %q, want location probe", r.URL.RawQuery)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			located = true
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<LocationConstraint>us-east-1</LocationConstraint>`))
		case http.MethodHead:
			checked = true
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<Error><Code>NoSuchBucket</Code><Message>missing</Message></Error>`))
		case http.MethodPut:
			created = true
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected MinIO request method %q", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	store, err := NewMinIOStore(context.Background(), strings.TrimPrefix(server.URL, "http://"), "access", "secret", "call-audio", false)
	if err != nil {
		t.Fatalf("NewMinIOStore() error = %v, want nil", err)
	}
	if store == nil || !located || !checked || !created {
		t.Errorf("NewMinIOStore() store=%v, located=%t, checked=%t, created=%t; want usable store after check/create", store != nil, located, checked, created)
	}
}
