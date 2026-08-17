# AI Call Analysis MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and verify a runnable Go backend MVP that uploads call audio, processes it through Redis/Asynq, produces a persisted transcript and structured analysis using Gemini or deterministic fake providers, calculates a backend-owned score out of 100, and exposes authenticated REST APIs.

**Architecture:** Use a modular monolith with `cmd/api`, `cmd/worker`, and `cmd/migrate`. Keep PostgreSQL, MinIO, Redis/Asynq, and Gemini behind focused package boundaries; keep scoring, state transitions, analysis validation, and RBAC independent of HTTP and external provider implementations.

**Tech Stack:** Go 1.23+, chi/v5, pgx/v5, PostgreSQL, go-redis/v9, Asynq, MinIO Go SDK, HMAC JWT, bcrypt, standard-library `log/slog`, Docker Compose, versioned embedded SQL migrations, and Go's standard testing package.

## Global Constraints

- Work only in `call_analyse_backend`; do not modify the frontend.
- Do not add Kubernetes, Kafka, Elasticsearch, CRM/telephony integrations, WebSockets, billing, SSO/OAuth providers, microservices, vector databases, or RAG.
- Use parameterized PostgreSQL queries and explicit transactions where multiple result rows must be persisted together.
- Use `context.Context` from HTTP requests and job handlers; provider calls must have explicit timeouts.
- Store passwords as bcrypt hashes and refresh tokens as hashes; never log or return secrets.
- Generate object keys on the backend; never use the uploaded filename as a filesystem path.
- Implement centralized call status transitions and reject invalid transitions.
- `AI_MODE=auto` uses Gemini only when a key exists and otherwise uses deterministic fake providers in development/test; production configuration must reject `auto` without a Gemini key and must reject `fake` in production.
- Follow red-green-refactor for production behavior: write a failing test, run it, implement the smallest passing change, then refactor while green.
- Do not create Git commits or push changes unless the user separately requests it.
- After each meaningful implementation session, update `docs/CURRENT_STATE.md`, append `docs/AGENT_LOG.md`, and create a factual `docs/agent-runs/YYYY-MM-DD-HHMM-<slug>.md` report.

## File map

The implementation will create these focused areas:

```text
cmd/api/main.go
cmd/worker/main.go
cmd/migrate/main.go
internal/config/
internal/database/
internal/auth/
internal/middleware/
internal/users/
internal/calls/
internal/storage/
internal/queue/
internal/transcription/
internal/analysis/
internal/scoring/
internal/dashboard/
internal/httpapi/
migrations/
tests/  (only if shared integration helpers are needed)
```

The root `migrations` package will export an embedded `fs.FS` so both the migration command and database tests can use the same SQL assets without copying them.

---

### Task 1: Bootstrap the Go module, configuration, entrypoints, and documentation harness

**Files:**
- Create: `go.mod`
- Create: `go.sum` via `go mod tidy`
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`
- Create: `cmd/api/main.go`
- Create: `cmd/worker/main.go`
- Create: `cmd/migrate/main.go`
- Create: `Dockerfile`
- Create: `docker-compose.yml`
- Create: `.env.example`
- Create: `Makefile`
- Create: `AGENTS.md`
- Create: `docs/SYSTEM.md`
- Create: `docs/ARCHITECTURE.md`
- Create: `docs/API.md`
- Create: `docs/DECISIONS.md`
- Create: `docs/CURRENT_STATE.md`
- Create: `docs/AGENT_LOG.md`
- Create: `docs/agent-runs/.gitkeep`

**Interfaces:**
- Produces `config.Config` and `config.Load() (Config, error)` for every runtime.
- Produces executable entrypoints that parse configuration and return a clear startup error until later dependencies are wired.

- [ ] **Step 1: Write failing configuration tests.**

Add tests for required secrets, default port/limits, parsing durations, and the production AI policy:

```go
func TestLoadRejectsProductionAutoModeWithoutGeminiKey(t *testing.T) {
    t.Setenv("APP_ENV", "production")
    t.Setenv("AI_MODE", "auto")
    t.Setenv("GEMINI_API_KEY", "")
    if _, err := Load(); err == nil {
        t.Fatal("expected production auto mode without key to fail")
    }
}
```

- [ ] **Step 2: Run the focused test and verify the expected missing-symbol failure.**

Run: `go test ./internal/config -run TestLoadRejectsProductionAutoModeWithoutGeminiKey -v`  
Expected: FAIL because `Load` and the package do not exist yet.

- [ ] **Step 3: Implement configuration loading.**

Define fields for app environment, HTTP port, database URL, Redis address, MinIO endpoint/credentials/bucket/TLS, JWT secrets and TTLs, Gemini key/models, AI mode, upload limit, provider timeout, and bootstrap admin credentials. Apply development-safe defaults, require JWT secrets outside tests, and enforce:

```text
production + AI_MODE=auto + empty GEMINI_API_KEY => error
production + AI_MODE=fake                    => error
AI_MODE=auto                                 => fake only outside production when key is empty
```

- [ ] **Step 4: Run the focused tests and then the package test.**

Run: `go test ./internal/config -v`  
Expected: PASS.

- [ ] **Step 5: Add minimal entrypoints and project harness files.**

Use `log/slog` JSON logging, keep the three `main.go` files small, create Compose services for API/worker/migrate/PostgreSQL/Redis/MinIO/minio-init, add health checks and a named volume for Postgres/MinIO, and add Make targets `up`, `down`, `logs`, `migrate-up`, `migrate-down`, `test`, `build`, `vet`, `fmt`, and `verify`.

Document the current empty implementation honestly and add the required agent instructions, system overview, architecture diagram, decisions, API placeholder policy, and log files.

- [ ] **Step 6: Verify the bootstrap deliverable.**

Run: `go test ./...`, `go build ./...`, `go vet ./...`, `docker compose config`, and `git diff --check`.  
Expected: Go commands pass; Compose parses; documentation contains no claim of unimplemented endpoints being ready.

---

### Task 2: Add migrations, database lifecycle, domain models, and call state transitions

**Files:**
- Create: `migrations/embed.go`
- Create: `migrations/000001_initial_schema.up.sql`
- Create: `migrations/000001_initial_schema.down.sql`
- Create: `internal/database/pool.go`
- Create: `internal/database/migrate.go`
- Create: `internal/calls/model.go`
- Test: `internal/calls/model_test.go`
- Create: `internal/scoring/model.go`
- Test: `internal/scoring/model_test.go`
- Modify: `cmd/migrate/main.go`

**Interfaces:**
- Produces `database.Open(ctx, cfg) (*pgxpool.Pool, error)` and `database.RunMigrations(ctx, pool, migrations.FS) error`.
- Produces `calls.Status`, `calls.Call`, `calls.CanTransition(from, to Status) bool`, and `calls.ValidateTransition(from, to Status) error`.
- Produces scoring criterion identifiers and maximum values used by parser, persistence, and tests.

- [ ] **Step 1: Write failing status-transition and criterion tests.**

Cover the valid path `uploaded → queued → transcribing → transcribed → analyzing → completed`, failure from any processing state, rejection of `completed → transcribing`, and every criterion maximum plus total maximum 100.

- [ ] **Step 2: Run the tests and confirm they fail for missing domain behavior.**

Run: `go test ./internal/calls ./internal/scoring -v`  
Expected: FAIL because the types and transition functions are not implemented.

- [ ] **Step 3: Implement the pure domain models and transition rules.**

Use UUID strings/`uuid.UUID`, nullable fields for duration/error, explicit status constants, and a transition table. Expose criterion metadata from one source of truth:

```go
type Criterion struct {
    Key string
    Max int
}

func CalculateTotal(scores map[string]int) (int, error)
```

- [ ] **Step 4: Run the tests and refactor only while green.**

Run: `go test ./internal/calls ./internal/scoring -v`  
Expected: PASS.

- [ ] **Step 5: Write the initial SQL migration.**

Create users, refresh tokens, calls, transcripts, analyses, and scores with UUID primary keys, foreign keys, unique `call_id` result constraints, status checks, score checks, timestamps, JSONB columns, and indexes for manager/status/created-at/result lookups. Add a migration table and make the migration runner idempotent.

- [ ] **Step 6: Implement pgx pool and embedded migration execution.**

Use `pgxpool.ParseConfig`, bounded acquire/connect timeouts, `migrations.FS`, and a transaction per migration. `cmd/migrate` must exit nonzero with a clear message when PostgreSQL is unavailable.

- [ ] **Step 7: Verify database assets.**

Run: `go test ./...`, `go build ./...`, `docker compose config`, and inspect `git diff --check`.  
Expected: all local checks pass and the SQL files contain no unparameterized runtime query.

---

### Task 3: Implement authentication, bootstrap admin, and RBAC policy

**Files:**
- Create: `internal/auth/password.go`
- Test: `internal/auth/password_test.go`
- Create: `internal/auth/jwt.go`
- Test: `internal/auth/jwt_test.go`
- Create: `internal/auth/model.go`
- Create: `internal/auth/store.go`
- Create: `internal/auth/service.go`
- Test: `internal/auth/service_test.go`
- Create: `internal/users/store.go`
- Create: `internal/middleware/auth.go`
- Create: `internal/middleware/rbac.go`
- Test: `internal/middleware/rbac_test.go`

**Interfaces:**
- Produces `auth.PasswordHasher`, `auth.TokenManager`, and `auth.Service` behavior for login/refresh/logout/me.
- Produces `auth.Claims` and authentication context helpers.
- Produces `auth.CanViewCall(role, authenticatedUserID, managerID string) bool` or equivalent policy function used by handlers and query scoping.

- [ ] **Step 1: Write failing password, token, and RBAC tests.**

Test that a bcrypt hash verifies without exposing the password, expired/wrong-secret JWTs are rejected, refresh token hashes are not accepted as access tokens, managers cannot view another manager's call, supervisors can view manager calls, and admins can view all calls.

- [ ] **Step 2: Run focused tests and verify expected failures.**

Run: `go test ./internal/auth ./internal/middleware -v`  
Expected: FAIL because authentication and policy functions are missing.

- [ ] **Step 3: Implement password hashing, JWT, refresh-token hashing, and policy functions.**

Use `bcrypt.GenerateFromPassword`, `github.com/golang-jwt/jwt/v5`, separate access/refresh secrets, explicit issuer/type claims, constant-time comparison for refresh hashes, and role constants. Keep token claims free of secrets and large payloads.

- [ ] **Step 4: Run focused tests and keep the implementation minimal.**

Run: `go test ./internal/auth ./internal/middleware -v`  
Expected: PASS.

- [ ] **Step 5: Implement PostgreSQL auth stores and service operations.**

Add parameterized queries for user lookup/creation, refresh-token insert/revoke/lookup, login, refresh rotation, logout, current-user lookup, and idempotent bootstrap admin creation. Enforce a transaction for refresh rotation so the old token is revoked when the replacement is persisted.

- [ ] **Step 6: Verify auth package compilation and security boundaries.**

Run: `go test ./...`, `go vet ./...`, and `go build ./...`.  
Expected: PASS; grep/log review confirms no password, JWT, refresh token, or API key is written to logs.

---

### Task 4: Implement MinIO storage and call upload/list/detail application services

**Files:**
- Create: `internal/storage/storage.go`
- Create: `internal/storage/minio.go`
- Test: `internal/storage/storage_test.go`
- Create: `internal/calls/store.go`
- Create: `internal/calls/service.go`
- Test: `internal/calls/service_test.go`
- Create: `internal/calls/validation.go`
- Test: `internal/calls/validation_test.go`

**Interfaces:**
- Produces `storage.ObjectStore` with `Put(ctx, key, reader, size, contentType) error`, `Get(ctx, key) (io.ReadCloser, error)`, and `Delete(ctx, key) error`.
- Produces `calls.Service.Create(ctx, actor, upload)`, `List(ctx, actor, page)`, and `Detail(ctx, actor, callID)` behavior.
- Produces `calls.ValidateUpload(filename, contentType string, size int64, maxBytes int64) error`.

- [ ] **Step 1: Write failing upload validation and ownership tests.**

Cover accepted `.mp3`, `.wav`, `.m4a`, rejected extensions/MIME mismatches, over-limit files, safe backend-generated object keys, manager ownership filtering, supervisor/admin visibility, and `CALL_NOT_FOUND` behavior.

- [ ] **Step 2: Run focused tests and verify red.**

Run: `go test ./internal/calls ./internal/storage -v`  
Expected: FAIL because validation, storage boundary, and service behavior do not exist.

- [ ] **Step 3: Implement upload validation, key generation, and in-memory test doubles.**

Normalize extensions case-insensitively, reject path separators and unsupported MIME types, use `http.MaxBytesReader`/multipart limits at the HTTP boundary, and generate keys such as `calls/<uuid>/<random-suffix>.<ext>` from server-owned identifiers.

- [ ] **Step 4: Run focused tests and verify green.**

Run: `go test ./internal/calls ./internal/storage -v`  
Expected: PASS.

- [ ] **Step 5: Implement MinIO and PostgreSQL call stores.**

Use the configured bucket, ensure/read bucket during startup, stream uploads without writing to a user-controlled filesystem path, insert calls after successful object upload, and delete the object if the database insert fails. List queries must apply actor scope before pagination and return total/page metadata.

- [ ] **Step 6: Verify the service package.**

Run: `go test ./...`, `go vet ./...`, and `go build ./...`.  
Expected: PASS.

---

### Task 5: Add transcript types, fake providers, Gemini adapters, and queue task definitions

**Files:**
- Create: `internal/transcription/model.go`
- Create: `internal/transcription/provider.go`
- Create: `internal/transcription/fake.go`
- Test: `internal/transcription/fake_test.go`
- Create: `internal/providers/gemini.go`
- Test: `internal/providers/gemini_test.go`
- Create: `internal/queue/tasks.go`
- Test: `internal/queue/tasks_test.go`

**Interfaces:**
- Produces:

```go
type AudioInput struct {
    Filename  string
    MIMEType  string
    Data      []byte
}

type TranscriptResult struct {
    Text     string
    Segments []Segment
}

type TranscriptionProvider interface {
    Transcribe(context.Context, AudioInput) (TranscriptResult, error)
}
```

- Produces `queue.NewProcessCallTask(callID string) (*asynq.Task, error)` and a stable task type name.
- Produces fake transcript/analysis implementations with deterministic outputs.

- [ ] **Step 1: Write failing fake-provider and queue-contract tests.**

Assert that fake transcription returns stable nonempty text and valid nullable segment timestamps, fake analysis returns all required fields and criterion keys, and task payload round-trips the call UUID.

- [ ] **Step 2: Run focused tests and verify red.**

Run: `go test ./internal/transcription ./internal/queue -v`  
Expected: FAIL because provider and task definitions are missing.

- [ ] **Step 3: Implement transcript models, deterministic fakes, and task construction.**

Keep speaker labels limited to `manager` and `client`; leave timestamps nil when unavailable. Use Asynq task payload JSON, a bounded retry count, and a uniqueness option keyed by call ID.

- [ ] **Step 4: Run focused tests and verify green.**

Run: `go test ./internal/transcription ./internal/queue -v`  
Expected: PASS.

- [ ] **Step 5: Write failing Gemini request/response tests.**

Use an `httptest.Server` to verify API-key placement, configured model selection, audio MIME/base64 encoding for transcription, JSON response extraction, provider timeout propagation, and non-2xx error sanitization. Do not call the real Gemini service.

- [ ] **Step 6: Implement the Gemini HTTP adapters.**

Keep Gemini-specific request structs in `internal/providers`. Use configured transcription and analysis model names, explicit HTTP client timeout, structured JSON prompting for analysis, and no transcript/API key logging. The adapter returns provider errors without exposing response bodies to API clients.

- [ ] **Step 7: Run provider tests and compile.**

Run: `go test ./internal/providers ./internal/transcription ./internal/queue -v`, `go vet ./...`, and `go build ./...`.  
Expected: PASS.

---

### Task 6: Implement strict analysis parsing and backend scoring

**Files:**
- Create: `internal/analysis/model.go`
- Create: `internal/analysis/parser.go`
- Test: `internal/analysis/parser_test.go`
- Create: `internal/analysis/provider.go`
- Create: `internal/scoring/calculator.go`
- Test: `internal/scoring/calculator_test.go`

**Interfaces:**
- Produces `analysis.ParseAndValidate(raw []byte) (Analysis, error)`.
- Produces `analysis.AnalysisProvider`:

```go
type AnalysisProvider interface {
    Analyze(context.Context, transcription.Transcript) (Analysis, error)
}
```

- Produces `scoring.Calculate(scores map[string]CriterionScore) (Score, error)` with criterion details and backend-owned total.

- [ ] **Step 1: Write failing parser tests.**

Cover valid JSON, invalid JSON, missing summary/next action, missing criterion, unknown criterion, negative score, score above criterion maximum, and a model-supplied `total_score` that must not influence the result.

- [ ] **Step 2: Run parser tests and verify red.**

Run: `go test ./internal/analysis -v`  
Expected: FAIL because parser and models are missing.

- [ ] **Step 3: Implement strict analysis models and validation.**

Decode with `encoding/json`, require the defined fields, validate criterion keys against `scoring.Criteria()`, reject invalid score ranges, retain raw JSON only for audit/debug persistence, and omit model totals from the domain model.

- [ ] **Step 4: Run parser tests and verify green.**

Run: `go test ./internal/analysis -v`  
Expected: PASS.

- [ ] **Step 5: Write failing scoring tests.**

Assert that all valid criterion scores sum correctly, a criterion score above max fails, negative values fail, and a calculated total cannot exceed 100 even if input contains an unrecognized extra field.

- [ ] **Step 6: Implement the pure calculator and analysis provider boundary.**

Calculate from the fixed eight criterion maxima; return the criterion details and total; never accept a caller-supplied total. Define the analysis provider interface next to its consumer boundary and adapt the fake/Gemini providers to it.

- [ ] **Step 7: Run scoring and all unit tests.**

Run: `go test ./... -v`.  
Expected: PASS.

---

### Task 7: Implement the worker pipeline, persistence, retries, and idempotency

**Files:**
- Create: `internal/calls/processing_store.go`
- Create: `internal/transcription/store.go`
- Create: `internal/analysis/store.go`
- Create: `internal/worker/processor.go`
- Create: `internal/worker/handler.go`
- Test: `internal/worker/processor_test.go`
- Modify: `cmd/worker/main.go`

**Interfaces:**
- Produces:

```go
type Processor struct {
    Calls        CallProcessingStore
    Transcripts  TranscriptStore
    Analyses     AnalysisStore
    Transcriber  transcription.TranscriptionProvider
    Analyzer     analysis.AnalysisProvider
    Objects      storage.ObjectStore
    ProviderTimeout time.Duration
}

func (p *Processor) Process(ctx context.Context, callID string) error
```

- Produces an Asynq handler that extracts the call ID, calls `Processor.Process`, and returns errors so Asynq retries.

- [ ] **Step 1: Write failing worker tests with in-memory stores and fake providers.**

Assert the exact success sequence `uploaded → transcribing → transcribed → analyzing → completed`, one transcript and one analysis/score, no second transcription when a transcript already exists, and `failed` plus sanitized error after a provider error.

- [ ] **Step 2: Run worker tests and verify red.**

Run: `go test ./internal/worker -v`  
Expected: FAIL because the processor and handler are missing.

- [ ] **Step 3: Implement the processor state machine.**

Load the call, inspect existing transcript state, use a bounded context timeout for provider calls, transition through the centralized transition function, persist transcript before analysis, parse/validate the analysis, calculate the score, and persist analysis plus score in one transaction. Sanitize provider errors to a stable message without raw credentials or full provider bodies.

- [ ] **Step 4: Run worker tests and verify green.**

Run: `go test ./internal/worker -v`  
Expected: PASS.

- [ ] **Step 5: Implement PostgreSQL result stores and Asynq server wiring.**

Use unique `call_id` constraints and upserts where duplicate delivery is possible. Wire Redis address, concurrency, retry count, queues, and task handler in `cmd/worker`. Use `signal.NotifyContext` and graceful shutdown for the Asynq server and database pool.

- [ ] **Step 6: Verify worker behavior and build.**

Run: `go test ./...`, `go vet ./...`, and `go build ./...`.  
Expected: PASS.

---

### Task 8: Implement HTTP middleware, auth/calls/dashboard handlers, and health endpoints

**Files:**
- Create: `internal/middleware/request_id.go`
- Create: `internal/middleware/logging.go`
- Create: `internal/httpapi/errors.go`
- Test: `internal/httpapi/errors_test.go`
- Create: `internal/httpapi/router.go`
- Create: `internal/httpapi/auth_handlers.go`
- Create: `internal/httpapi/call_handlers.go`
- Test: `internal/httpapi/call_handlers_test.go`
- Create: `internal/httpapi/dashboard_handlers.go`
- Create: `internal/httpapi/health_handlers.go`
- Create: `internal/dashboard/store.go`
- Create: `internal/dashboard/service.go`
- Modify: `cmd/api/main.go`

**Interfaces:**
- Produces an `http.Handler` from `httpapi.NewRouter(deps)` with all documented routes.
- Produces consistent JSON error responses and request IDs.
- Produces dashboard summary behavior scoped through the authenticated actor.

- [ ] **Step 1: Write failing handler tests.**

Use `httptest` with service fakes to cover login/refresh/logout/me routing, unauthenticated rejection, multipart upload success and validation errors, call list/detail authorization, consistent `CALL_NOT_FOUND` errors, health live/ready responses, and dashboard scoping.

- [ ] **Step 2: Run HTTP tests and verify red.**

Run: `go test ./internal/httpapi -v`  
Expected: FAIL because router, middleware, and handlers are missing.

- [ ] **Step 3: Implement middleware and error encoding.**

Add request IDs, structured request logging with safe fields, bearer-token parsing, actor context, role middleware, JSON decoding helpers, multipart size limits, and a single error writer that maps domain errors to status/code pairs without internal details.

- [ ] **Step 4: Run error/middleware tests and verify green.**

Run: `go test ./internal/httpapi -run 'TestError|TestMiddleware' -v`  
Expected: PASS.

- [ ] **Step 5: Implement auth, calls, dashboard, and health routes.**

Register the exact documented paths. Return `201` for accepted uploads, `200` for reads/auth/health, `401` for missing/invalid auth, `403` for insufficient role/ownership, `404` for missing calls, `400` for invalid input, and `202` only if the upload response explicitly represents asynchronous acceptance; keep the API contract consistent with the documented response examples.

- [ ] **Step 6: Run all HTTP tests and integrate with real services.**

Run: `go test ./internal/httpapi ./internal/dashboard -v`, `go test ./...`, and `go build ./...`.  
Expected: PASS.

---

### Task 9: Complete Docker runtime, migration startup, and local smoke flow

**Files:**
- Modify: `Dockerfile`
- Modify: `docker-compose.yml`
- Modify: `Makefile`
- Modify: `.env.example`
- Modify: `README.md`
- Modify: `docs/API.md`
- Modify: `docs/ARCHITECTURE.md`

**Interfaces:**
- Produces a Compose stack where `migrate` completes before API/worker, MinIO bucket initialization is automatic, and API/worker can use fake AI with no Gemini credentials.

- [ ] **Step 1: Add a configuration and Compose validation check.**

Run: `docker compose config` with the example environment.  
Expected: PASS and services expose only the intended development ports.

- [ ] **Step 2: Build the multi-target image.**

Use a Go builder stage to compile `api`, `worker`, and `migrate`, then use a small runtime image with each Compose service selecting its binary. Do not copy `.env` or source secrets into the image.

- [ ] **Step 3: Start the stack and verify health.**

Run: `docker compose up --build -d`, then poll `GET http://localhost:8080/health/live` and `GET http://localhost:8080/health/ready`.  
Expected: migration exits successfully, MinIO bucket exists, API and worker remain running, and readiness reports all required dependencies available.

- [ ] **Step 4: Run the fake-provider smoke flow.**

Bootstrap the documented admin, log in, upload a small valid audio file, poll `GET /api/v1/calls/{id}`, and confirm the status reaches `completed` with transcript, analysis, and a score between 0 and 100. Use a generated fixture only for the local smoke test; do not commit an audio file containing real people or secrets.

- [ ] **Step 5: Update README and API documentation with observed behavior.**

Document environment setup, Compose startup, migration behavior, bootstrap admin, worker startup, test commands, curl login/upload/detail examples, fake-provider mode, and the fact that real Gemini was not exercised without credentials. Document only routes that exist.

---

### Task 10: Final verification, documentation snapshot, and factual run report

**Files:**
- Modify: `docs/CURRENT_STATE.md`
- Modify: `docs/AGENT_LOG.md`
- Create: `docs/agent-runs/2026-08-14-1537-ai-call-analysis-mvp.md` using the actual `HHMM` time when the implementation session runs.
- Modify: `docs/DECISIONS.md`
- Modify: `docs/SYSTEM.md`

- [ ] **Step 1: Run the complete verification set.**

Run each command fresh and record its actual exit status/output summary:

```text
gofmt -w $(rg --files -g '*.go')
go test ./...
go vet ./...
go build ./...
docker compose config
git diff --check
git status --short
```

On Windows PowerShell, use `Get-ChildItem -Recurse -Filter *.go | ForEach-Object { gofmt -w $_.FullName }` for formatting rather than relying on shell substitution.

- [ ] **Step 2: Re-run tests after formatting.**

Run: `go test ./...` and `go vet ./...`.  
Expected: PASS with no new warnings.

- [ ] **Step 3: Update the current-state snapshot.**

List implemented features, partial features, out-of-scope items, known limitations, whether Docker smoke ran, whether real Gemini ran, and the exact final verification results. Do not claim unexecuted checks passed.

- [ ] **Step 4: Append the agent log and run report.**

Record the requested goal, initial clean repository state, files changed, migrations, tests, commands, results, discovered issues, intentionally unaddressed limitations, and recommended next step. Keep previous `AGENT_LOG.md` entries intact.

- [ ] **Step 5: Perform the final requirement checklist.**

Confirm the definition of done item-by-item: Docker infrastructure, migrations, bootstrap auth, RBAC, MinIO upload, Redis/Asynq job, worker, provider interfaces, transcript, analysis, backend score, scoped detail/dashboard, tests, build, and documentation harness. If any item is missing, report `PARTIAL` and classify the reason rather than claiming completion.

## Plan self-review

- **Spec coverage:** Tasks 1–2 cover bootstrap, Docker/config, migrations, health scaffolding, entities, indexes, and state transitions. Tasks 3–4 cover auth/RBAC and uploads/MinIO/calls. Tasks 5–7 cover fake/Gemini providers, queue, worker, transcript, analysis, scoring, retries, and idempotency. Task 8 covers REST and dashboard. Tasks 9–10 cover Docker smoke, README, documentation harness, and verification.
- **Production safety:** The production AI-mode guard is explicit in Task 1 and tested before implementation. Secrets, object keys, SQL parameterization, request context, provider timeouts, and sanitized errors are repeated in relevant tasks.
- **Type consistency:** `config.Load`, `storage.ObjectStore`, `TranscriptionProvider`, `AnalysisProvider`, `Processor.Process`, scoring criteria, and router dependencies are defined before downstream tasks consume them.
- **No placeholders:** The plan does not defer requirements or hide ambiguity; when an aggregate is intentionally bounded, the plan requires documenting the observed limitation.
- **Commit policy:** The generic plan template requests commits, but this repository's user instruction explicitly prohibits commits without separate authorization, so this plan omits commit steps.
