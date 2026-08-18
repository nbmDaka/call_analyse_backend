// Package storage defines the object-storage boundary used by call services.
package storage

import (
	"context"
	"io"
)

// ObjectStore streams objects to and from object storage without exposing a
// filesystem path to callers.
type ObjectStore interface {
	Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}
