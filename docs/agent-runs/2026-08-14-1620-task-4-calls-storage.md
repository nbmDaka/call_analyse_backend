# Task 4 calls and storage run

## Requested task

Implement MinIO storage, audio upload validation, and scoped call application services.

## Initial repository state

Tasks 1–3 were present in the working tree. No Git commit or push was created.

## Implementation

- Added `internal/storage/storage.go` and `internal/storage/minio.go`.
- Added `internal/calls/validation.go`, `internal/calls/service.go`, and `internal/calls/store.go`.
- Added focused validation, storage, and call-service tests.

## Verification

- `go test ./internal/calls ./internal/storage -v` — PASS.
- `go test ./...` — PASS.
- `go vet ./...` — PASS.
- `go build ./...` — PASS.
- `docker compose config` with documented development values — PASS.

## Limitations

Docker runtime and live MinIO/PostgreSQL integration were not run. `git diff --check` reports only pre-existing whitespace in the approved plan/spec documents.
