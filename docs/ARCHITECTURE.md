# Architecture

The MVP is a modular monolith with focused package boundaries and three executable
processes. API and worker share PostgreSQL, MinIO, Redis/Asynq, and provider boundaries;
migrations run as a short-lived Compose dependency.

```text
Client
  |
  v
API ---- PostgreSQL
 |  \\---- MinIO
 \\----- Redis/Asynq ---- Worker ---- MinIO
                           |
                           v
                Gemini or deterministic fake providers
```

`internal/config`, `internal/database`, `internal/calls`, `internal/scoring`,
`internal/auth`, `internal/users`, `internal/middleware`, `internal/storage`,
`internal/queue`, `internal/transcription`, `internal/analysis`, `internal/worker`,
`internal/dashboard`, and `internal/httpapi` are implemented. API startup wires the
real services and gracefully shuts down; worker startup owns asynchronous processing
and persisted checkpoints.
