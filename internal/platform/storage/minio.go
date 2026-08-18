package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOStore implements ObjectStore through a single configured MinIO bucket.
type MinIOStore struct {
	client *minio.Client
	bucket string
}

// NewMinIOStore creates a MinIO-backed object store and ensures its configured
// bucket exists before returning it for use.
func NewMinIOStore(ctx context.Context, endpoint, accessKey, secretKey, bucket string, useSSL bool) (*MinIOStore, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, fmt.Errorf("MinIO endpoint is required")
	}
	if strings.TrimSpace(bucket) == "" {
		return nil, fmt.Errorf("MinIO bucket is required")
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create MinIO client: %w", err)
	}
	store := &MinIOStore{client: client, bucket: bucket}
	if err := store.EnsureBucket(ctx); err != nil {
		return nil, fmt.Errorf("ensure MinIO bucket: %w", err)
	}
	return store, nil
}

// EnsureBucket verifies that the configured bucket exists and creates it when absent.
func (s *MinIOStore) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("check MinIO bucket: %w", err)
	}
	if exists {
		return nil
	}
	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("create MinIO bucket: %w", err)
	}
	return nil
}

// Ready verifies that the configured bucket remains reachable after startup.
func (s *MinIOStore) Ready(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("MinIO bucket %q does not exist", s.bucket)
	}
	return nil
}

// Put streams an object directly to MinIO.
func (s *MinIOStore) Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	if _, err := s.client.PutObject(ctx, s.bucket, key, reader, size, minio.PutObjectOptions{ContentType: contentType}); err != nil {
		return fmt.Errorf("put MinIO object: %w", err)
	}
	return nil
}

// Get opens a streaming object reader. The caller must close the reader.
func (s *MinIOStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get MinIO object: %w", err)
	}
	return object, nil
}

// Delete removes an object from the configured MinIO bucket.
func (s *MinIOStore) Delete(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete MinIO object: %w", err)
	}
	return nil
}

var _ ObjectStore = (*MinIOStore)(nil)
