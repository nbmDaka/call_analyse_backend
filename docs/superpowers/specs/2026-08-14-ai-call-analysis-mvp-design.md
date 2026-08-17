# AI Call Analysis MVP Design

**Date:** 2026-08-14  
**Repository:** `call_analyse_backend`

## Goal

Build a runnable backend MVP for uploading sales-call audio, processing it asynchronously, transcribing and analyzing it through replaceable AI providers, calculating a backend-owned score out of 100, and exposing the result through a REST API.

The first implementation is a vertical slice. Frontend, CRM, telephony integrations, and other explicitly out-of-scope product surfaces are not included.

## Selected approach

Use a modular monolith with two long-running Go processes and one short-lived migration process:

- `cmd/api` serves the HTTP API.
- `cmd/worker` consumes Redis/Asynq jobs.
- `cmd/migrate` applies embedded versioned SQL migrations.

The application will use `chi`, `pgx/v5`, Redis with Asynq, MinIO's S3-compatible API, and a small Gemini HTTP adapter. External systems are hidden behind focused boundaries; business logic does not depend directly on HTTP handlers or Gemini implementation details.

This approach is preferred over an ORM or generated query layer for the MVP because it keeps the dependency graph and runtime behavior explicit while retaining parameterized SQL, transactions, and testable interfaces.

## Runtime architecture

```text
Client
  |
  v
Go API ---- PostgreSQL
  |  \------ MinIO
  \--------- Redis/Asynq ---- Go Worker ---- MinIO
                                  |             |
                                  v             v
                         TranscriptionProvider  AnalysisProvider
                                  |             |
                                  +------ Gemini or deterministic fake
```

The API accepts an audio upload, validates it, stores it under a backend-generated MinIO object key, creates the call row, and enqueues one `process_call` job. The request never waits for AI work.

The worker executes the pipeline:

```text
uploaded → queued → transcribing → transcribed → analyzing → completed
```

Any processing error is sanitized, stored in `calls.error_message`, and moves the call to `failed`. A retry resumes from the useful persisted stage: transcription is repeated only when no transcript exists; analysis can be retried without retranscribing an existing transcript.

## Package boundaries

- `internal/config`: environment parsing, defaults, and validation.
- `internal/database`: pgx pool setup, migration runner integration, and transaction helpers.
- `internal/auth`: password hashing, JWT access tokens, refresh-token lifecycle, bootstrap admin, and auth service behavior.
- `internal/middleware`: request ID, structured request logging, authentication context, and role checks.
- `internal/calls`: call models, status-transition rules, upload/detail/list behavior, and ownership checks.
- `internal/storage`: object storage interface and MinIO implementation.
- `internal/queue`: Asynq task type, enqueue options, retries, and uniqueness.
- `internal/transcription`: transcript types and transcription provider boundary.
- `internal/analysis`: structured analysis types, JSON parsing/validation, analysis persistence, and worker-facing orchestration.
- `internal/scoring`: pure criterion validation and 0–100 score calculation.
- `internal/dashboard`: scoped aggregate queries for summary statistics.
- `internal/httpapi`: route registration, request decoding, response encoding, and consistent errors.

Interfaces are introduced at external-system and worker-test boundaries only. The implementation will avoid a generic repository/service/factory layer for every function.

## AI provider behavior

The configuration exposes `AI_MODE=auto|gemini|fake`.

- `auto` selects Gemini when `GEMINI_API_KEY` is present and the deterministic fake provider otherwise.
- `gemini` fails clearly at startup or job execution when credentials are missing.
- `fake` produces stable transcript and analysis data for local development and automated tests.

The transcription and analysis stages remain separate. The transcription provider receives audio bytes and metadata and returns full text plus optional speaker segments. Missing timestamps remain nullable; the system does not invent timestamps. The analysis provider receives the persisted transcript and returns structured fields for summary, needs, objections, refusal reason, mistakes, strengths, next action, and criterion results.

Gemini responses are parsed as JSON and validated before persistence. The backend rejects malformed JSON, missing required fields, unknown or invalid criterion values, criterion scores above their maxima, and totals outside 0–100. Any model-provided total is ignored.

## Authentication and RBAC

Passwords are stored only as bcrypt hashes. Access tokens are short-lived HMAC JWTs. Refresh tokens are longer-lived, stored in the database only as hashes, and revocable through logout or token rotation.

Bootstrap admin creation is idempotent and driven by `BOOTSTRAP_ADMIN_EMAIL` and `BOOTSTRAP_ADMIN_PASSWORD`. Existing users are not overwritten.

Authorization is enforced in backend queries and service logic:

- `admin`: all users, calls, analyses, scores, and dashboard data.
- `supervisor`: calls and analytics for managers.
- `manager`: only calls, transcripts, analyses, and scores where `manager_id` is the authenticated user.

## Data model and migrations

Versioned migrations create:

- `users`
- `refresh_tokens`
- `calls`
- `call_transcripts`
- `call_analyses`
- `call_scores`

UUIDs are used for identifiers. Call status is constrained to the supported states. One transcript, analysis, and score row is associated with each call through unique `call_id` constraints. Score columns have database and application validation for their criterion maxima and a total range of 0–100.

Indexes cover `calls.manager_id`, `calls.status`, `calls.created_at`, and each result table's `call_id`. AI arrays and objection details are stored as JSONB rather than prematurely normalized into many tables.

## REST API

Health endpoints:

```text
GET /health/live
GET /health/ready
```

Authentication endpoints:

```text
POST /api/v1/auth/login
POST /api/v1/auth/refresh
POST /api/v1/auth/logout
GET  /api/v1/me
```

Call endpoints:

```text
POST /api/v1/calls
GET  /api/v1/calls
GET  /api/v1/calls/{id}
```

`POST /calls` accepts `multipart/form-data`, validates `.mp3`, `.wav`, and `.m4a`, enforces the configured size limit, and returns the new call with `queued` status. Generated object keys never use the client filename as a filesystem path.

`GET /calls` supports pagination and scopes rows according to RBAC. Call detail returns the call, manager, audio metadata, transcript, analysis, and score when present.

Dashboard endpoint:

```text
GET /api/v1/dashboard/summary
```

It returns scoped total/completed/failed call counts and average score. Common mistakes and refusal reasons are included when the aggregate query remains within the MVP boundary; otherwise the limitation is documented rather than expanding scope.

All API errors use the form:

```json
{
  "error": {
    "code": "CALL_NOT_FOUND",
    "message": "call not found"
  }
}
```

Internal stack traces and secrets are never returned or logged.

## Queue and failure handling

The API enqueues a single `process_call` Asynq task with a bounded retry policy, task uniqueness, and a call ID payload. The worker uses request-independent job context with explicit provider timeouts.

Worker persistence is idempotent:

- status updates are centralized and reject invalid transitions;
- transcript, analysis, and score writes use unique call IDs and upsert/transaction behavior;
- an existing transcript allows analysis retry without a new transcription;
- an existing completed result is not recreated on duplicate delivery.

Worker and API logs include request ID, call ID, job ID, user ID, status transitions, provider errors, and retry information. They never include passwords, JWTs, refresh tokens, API keys, or full transcripts by default.

## Docker and configuration

Docker Compose provisions:

```text
api
worker
migrate
postgres
redis
minio
minio-init
```

PostgreSQL, Redis, and MinIO receive health checks where meaningful. `minio-init` creates the `call-audio` bucket automatically. Configuration is supplied through environment variables and documented in `.env.example`; real secrets are not committed.

## Testing strategy

Implementation follows red-green-refactor cycles. Critical unit tests cover:

- criterion validation and total score calculation;
- score maximum and 100-point bounds;
- manager/supervisor/admin call visibility;
- valid and invalid structured analysis JSON;
- required analysis fields and score validation;
- worker success sequence from upload through completion;
- provider failure resulting in a failed call;
- idempotent result persistence where practical.

Worker tests use fake providers and in-memory test doubles. No real Gemini credentials are required for the unit suite. Docker/configuration verification is performed separately when the environment provides Docker.

## Documentation harness

The repository will contain `AGENTS.md` and the required `docs/` files:

- `SYSTEM.md`
- `ARCHITECTURE.md`
- `API.md`
- `DECISIONS.md`
- `CURRENT_STATE.md`
- `AGENT_LOG.md`
- `agent-runs/YYYY-MM-DD-HHMM-<slug>.md`

Future coding agents must read the system, architecture, current-state, decisions, and agent-log files before making changes; update affected documentation and append factual run records after changes; and record actual verification outcomes. No previous log entries may be deleted or rewritten.

## Out of scope

This design does not include React, CRM or telephony integrations, WebSockets, Kafka, Kubernetes, multi-region deployment, billing, payments, email, notifications, SSO/OAuth providers, complex organization hierarchies, microservices, vector databases, or RAG.

## Success criteria

The MVP is considered implemented when Docker Compose starts the infrastructure and both Go processes, migrations apply, bootstrap admin can log in, an audio file can be uploaded to MinIO, a Redis/Asynq job is processed by the worker, fake or Gemini providers produce a persisted transcript and analysis, backend scoring produces a 0–100 result, RBAC scopes calls correctly, detail and dashboard endpoints work, tests pass, and documentation records the actual state and verification results.
