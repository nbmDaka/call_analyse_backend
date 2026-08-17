package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"call_analyse_backend/internal/config"
)

func TestInitializeStorageEnsuresConfiguredBucket(t *testing.T) {
	var locationProbe, bucketCheck, bucketCreate bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Trim(r.URL.Path, "/") != "call-audio" {
			t.Errorf("request path = %q, want configured bucket", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodGet:
			locationProbe = r.URL.Query().Has("location")
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<LocationConstraint>us-east-1</LocationConstraint>`))
		case http.MethodHead:
			bucketCheck = true
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<Error><Code>NoSuchBucket</Code><Message>missing</Message></Error>`))
		case http.MethodPut:
			bucketCreate = true
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected MinIO request method %q", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	cfg := config.Config{
		MinIOEndpoint:  strings.TrimPrefix(server.URL, "http://"),
		MinIOAccessKey: "access",
		MinIOSecretKey: "secret",
		MinIOBucket:    "call-audio",
	}
	if err := initializeStorage(context.Background(), cfg); err != nil {
		t.Fatalf("initializeStorage() error = %v, want nil", err)
	}
	if !locationProbe || !bucketCheck || !bucketCreate {
		t.Errorf("initializeStorage() locationProbe=%t bucketCheck=%t bucketCreate=%t; want complete bucket initialization", locationProbe, bucketCheck, bucketCreate)
	}
}
