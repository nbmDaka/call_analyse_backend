# Current state

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
  provider errors; unit tests use only local `httptest` servers.
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

- Docker runtime smoke verification remains pending because the Docker Desktop
  Linux engine is unavailable in this environment.
- README refresh is pending because the existing file contains invalid UTF-8 for
  the repository patch tool; API and architecture docs are current.
- Live PostgreSQL/MinIO/Redis integration and real Gemini execution remain
  unverified; unit tests use fakes and local HTTP test servers.
- Worker memory usage is bounded by the configured upload limit but still reads
  one audio object into memory for provider calls; scale-out worker coordination
  remains process-local in this MVP.

## Verification status

Task 8 focused HTTP/dashboard tests and the full Go test, vet, and build checks
passed. No Docker runtime smoke test, live PostgreSQL or MinIO integration, Redis
enqueue, or real Gemini call has been performed. `git diff --check` still reports
pre-existing trailing whitespace in the approved plan/spec documents.
