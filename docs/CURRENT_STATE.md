# Current state

## Multi-tenant workspace implementation (2026-08-21)

- Added personal/company workspaces and independent owner/admin/supervisor/manager
  memberships, including current-state actor resolution on every workspace request.
- Registration atomically creates a user, personal workspace, and owner membership.
- Added explicit workspace calls/dashboard routes, membership management, platform
  user/workspace administration, system metrics, and audited platform mutations.
- Calls now persist workspace, owner, and uploader IDs; new MinIO keys and Asynq
  payloads include the workspace ID. Existing object keys remain readable.
- Manager, supervisor, workspace admin/owner SQL scopes apply before pagination,
  detail selection, and dashboard aggregation. Cross-tenant detail is hidden as 404.
- The React client selects and persists an active workspace, uses tenant-aware query
  keys, and provides members and platform administration views in Russian/Kazakh.
- Migration 000003 creates and backfills the tenant model while retaining deprecated
  `users.role`, `users.supervisor_id`, and `calls.manager_id` for rollback compatibility.

## Implemented in Tasks 1-9

- Go module and environment-based configuration contract.
- Production-safe AI mode validation and development/test fake fallback.
- API, worker, and migration command scaffolds with JSON structured startup errors.
- Docker, Compose, Make, environment-example, and documentation scaffolding.
- Embedded, versioned PostgreSQL schema migration assets and an idempotent,
  transaction-per-migration runner.
- Bounded pgx pool opening used by the migration command.
- Pure call lifecycle transition rules and the eight 100-point scoring criteria.
- Bcrypt/JWT authentication primitives, refresh-token rotation/revocation stores,
  bootstrap-admin service, and manager/supervisor/admin RBAC policy.
- MinIO object-storage boundary and backend-generated audio object keys.
- Audio extension/MIME/size validation and scoped call upload/list/detail services.
- API startup now initializes the configured MinIO bucket with a bounded timeout
  before later runtime wiring is attempted.
- Transcript models with nullable segment timestamps and deterministic fake
  transcription output.
- Shared analysis-provider contract plus deterministic fake analysis output for
  local development and worker tests.
- Gemini HTTP adapters with configured models, explicit timeouts, and sanitized
  provider errors; unit tests use only local `httptest` servers. The default
  `gemini-3.7-flash` model handles both audio transcription and structured analysis.
- Stable Asynq `process_call` task construction with UUID payload validation,
  bounded retries, queue selection, and payload-derived uniqueness.
- Strict analysis JSON parsing and backend-owned 0–100 score calculation are
  implemented as pure, testable logic.
- HTTP request IDs, safe structured request logging, bearer authentication,
  RBAC-aware routes, consistent JSON errors, health endpoints, call upload/list/
  detail, auth, dashboard summary, and pagination are implemented.
- API startup wires PostgreSQL, MinIO, auth/bootstrap admin, Redis/Asynq enqueue,
  dashboard, and graceful HTTP shutdown.
- Call detail reads persisted manager, audio metadata, transcript, analysis, and
  analysis criterion feedback, and backend-owned score when asynchronous
  checkpoints exist.
- Compose orders migration before API/worker, initializes the MinIO bucket, and
  documents the fake-provider development path; `docker compose config` passes.

## Not implemented

- README refresh is pending because the existing file contains invalid UTF-8 for
  the repository patch tool; API and architecture docs are current.
- Real Gemini execution remains unverified. The multi-tenant Docker smoke used the
  deterministic development provider and isolated temporary infrastructure.
- Worker memory usage is bounded by the configured upload limit but still reads
  one audio object into memory for provider calls; scale-out worker coordination
  remains process-local in this MVP.
- Platform support access to transcript/audio content is intentionally not exposed.
  A future explicit support flow must write an audit event before returning content;
  ordinary platform endpoints never grant silent workspace access.
- Invitation delivery/acceptance and team tables are future extensions; membership
  creation currently activates an existing account directly.

## Verification status

The full Go test, vet, and build checks pass. Migration 000003 passed against seeded
legacy data in isolated PostgreSQL 16. An isolated Compose project brought up API,
worker, PostgreSQL, Redis, and MinIO; readiness, platform company creation, and its
audit event were verified. No real Gemini call was performed.

The companion `call_analyse_frontend` repository now consumes the implemented auth,
dashboard, calls, upload, and call-detail API vertical slice. Vitest, TypeScript,
production build, and 22 Chromium Playwright E2E scenarios pass, including a tenant
switch cache-isolation scenario. The in-app browser was unavailable, so no separate
manual in-app smoke was performed. The MVP client uses localStorage for its token
session and polls non-terminal call details.
The API also exposes a configured CORS allowlist for the separate Vite development
origin, and the frontend deduplicates concurrent refresh requests.
