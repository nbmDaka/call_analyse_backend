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

Tenant authorization is rebuilt for every `/workspaces/{workspaceID}/...` request
by joining the authenticated user to the current workspace and membership rows.
The resulting actor is passed through handlers, services, and stores. Call list,
detail, pagination, and dashboard SQL apply `workspace_id` before role scope,
pagination, or aggregation. Supervisor scope uses same-workspace membership IDs.

`internal/modules/workspaces`, `memberships`, and `platform` own tenant lifecycle,
membership management, and platform administration. Platform mutations are separate
from workspace endpoints and write sanitized `audit_events`. New Asynq tasks contain
both workspace and call IDs; the Gemini provider remains unaware of authorization.

The companion `call_analyse_frontend` repository is a React/TypeScript client using
the REST API, TanStack Query for server state, and React state for local UI state.
