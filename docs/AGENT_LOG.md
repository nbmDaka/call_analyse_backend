# Agent log

This append-only log records factual implementation sessions. Task 1's detailed run
record will be stored in `docs/agent-runs/` and the requested implementer report.

## 2026-08-21 16:58 — Multi-tenant workspace architecture

- Added versioned workspace/membership/audit schema and a legacy-data backfill with
  required call tenant, owner, and uploader fields while retaining compatibility columns.
- Added database-backed workspace actors, membership and platform services, explicit
  workspace routes, tenant-scoped calls/dashboard SQL, and workspace-bound worker tasks.
- Registration now atomically creates personal workspace ownership; platform and
  workspace roles are independent and suspended/disabled access fails closed.
- Migrated the React client to persisted workspace selection, tenant query keys,
  role-aware navigation, members management, and platform administration in RU/KK.
- Isolated PostgreSQL migration and Compose runtime smoke passed. Go tests/vet/build,
  frontend tests/typecheck/build, and 22 Chromium Playwright E2E scenarios passed.
- No Git commit or push was created.

## 2026-08-14 15:48 — Task 1 bootstrap

- Added the Go module, TDD-backed configuration loader, command scaffolds, Docker/
  Compose shape, environment example, Make targets, and documentation harness.
- Verified `go test ./...`, `go build ./...`, `go vet ./...`, `docker compose config`,
  and `git diff --check` with exit code 0 before writing the session records.
- No Docker runtime smoke flow or real Gemini request was run. The three entrypoints
  intentionally emit JSON startup errors until later runtime wiring tasks exist.
- No Git commit or push was created.

## 2026-08-17 — Final full-stack review fixes

- Review found and fixed concurrent refresh-token rotation races in the frontend API
  client by sharing one in-flight refresh promise across parallel 401 responses.
- Review found and fixed separate-dev-origin browser access by adding a configurable
  backend CORS allowlist and local Vite-origin defaults, including preflight tests.
- Supervisor upload actions are now hidden and direct upload navigation redirects to
  calls; manager and admin retain upload access consistent with backend policy.
- Login return paths are restricted to safe internal paths to avoid open-redirect
  behavior through crafted navigation state.
- No unrelated refactor, commit, or push was performed.

## 2026-08-17 — Frontend MVP vertical slice

- Implemented the companion React/TypeScript/Vite frontend with authenticated session
  handling, dashboard, calls list, audio upload, status polling, call detail, score,
  criteria, analysis insights, and transcript views.
- Added API-client normalization for both frontend-style and Go default exported-field
  response names, plus refresh-once behavior after a 401 response.
- Frontend `npm test` passed (3 tests), `npm run typecheck` passed, and `npm run build`
  passed. Browser smoke testing was not run.
- No Git commit or push was created.

## 2026-08-14 18:30 — Task 8 HTTP and API runtime

- Added request-ID/logging middleware, sanitized JSON errors, auth, call upload/list/detail,
  dashboard summary, health routes, and scoped HTTP tests.
- Added API runtime wiring for PostgreSQL, MinIO, auth/bootstrap admin, Redis/Asynq enqueue,
  dashboard, graceful HTTP shutdown, and full call-detail checkpoint reads.
- Added multipart body limits, pagination validation, nil dependency guards, and compensation
  when queue submission fails after upload persistence.
- `go test ./...`, `go vet ./...`, and `go build ./...` passed. Scoped Task 8 review approved.
- Docker/live infrastructure and Gemini execution remain unverified. No Git commit or push.

## 2026-08-14 18:45 — Task 9 Compose harness

- Added migration completion ordering for API/worker, corrected migration Make targets,
  set a local bootstrap example, and updated API/architecture documentation.
- `docker compose --env-file .env.example config` passed.
- Docker build/up/health/upload smoke could not run: Docker Desktop Linux engine was
  unavailable (`//./pipe/dockerDesktopLinuxEngine` missing). README remains a documented
  follow-up because its existing bytes are not valid UTF-8 for the patch tool.
- No Git commit or push was created.

## 2026-08-14 16:30 — Task 4 MinIO startup wiring

- Coupled MinIO store construction to bucket ensure behavior and wired the API
  startup scaffold to initialize the configured bucket with a bounded timeout.
- Added API/storage focused tests; `go test ./...`, `go vet ./...`, and `go build ./...`
  passed. Live MinIO runtime was not run.
- No Git commit or push was created.

## 2026-08-14 16:05 — Task 2 database lifecycle and domain primitives

- Added root-embedded initial schema migrations, a bounded pgx pool opener, and an
  idempotent transaction-per-migration runner wired into `cmd/migrate`.
- Added tested call status transitions and centralized eight-criterion score metadata
  totaling 100.
- Verified the focused RED/GREEN cycle, `go test ./...`, `go build ./...`,
  `go vet ./...`, and `docker compose config` with temporary development values.
- `git diff --check` still reports only pre-existing trailing whitespace in the Task 1
  plan/spec documents; no Task 2 files were changed to resolve that unrelated issue.
- No Git commit or push was created.

## 2026-08-14 16:11 — Task 2 fix round 1: scoring contract

- Corrected the eight scoring identifiers, maxima, and initial-schema score checks;
  removed `listening` and `next_steps` in favor of `rapport` and `next_action`.
- Recorded RED/GREEN evidence with an exact scoring-contract test, then verified
  `go test ./...`, `go build ./...`, and `go vet ./...` with exit code 0.
- No Git commit or push was created.

## 2026-08-14 16:20 — Task 4 calls and storage

- Added MinIO object storage boundary/adapter, safe audio validation, backend-generated
  object keys, and scoped call upload/list/detail services with parameterized SQL.
- Focused calls/storage tests, `go test ./...`, `go vet ./...`, `go build ./...`, and
  Compose configuration parsing passed.
- Docker runtime and live MinIO/PostgreSQL integration were not run; no real Gemini
  request was made.
- No Git commit or push was created.

## 2026-08-14 17:10 — Task 5 providers and queue contracts

- Added transcript/provider contracts, deterministic fake transcription and analysis
  providers, Gemini HTTP adapters, and stable Asynq process-call task creation.
- Gemini adapter tests used local `httptest` servers only; no API key, transcript,
  or provider response body is logged or exposed by adapter errors.
- Focused and full Go tests, vet, and build passed. Compose configuration was checked
  with `.env.example`; no Docker runtime, Redis enqueue, or real Gemini request ran.
- `git diff --check` continues to report unrelated trailing whitespace in the
  existing plan/spec documents.
- No Git commit or push was created.

## 2026-08-14 17:25 — Task 6 analysis and scoring

- Added strict analysis JSON validation and pure backend-owned scoring over the exact
  eight criteria; model-provided totals are ignored.
- Focused analysis/scoring tests, `go test ./...`, `go vet ./...`, and `go build ./...`
  passed. No real Gemini or database persistence was run.
- No Git commit or push was created.

## 2026-08-21 16:19 — Gemini model refresh

- Replaced the retired `gemini-2.0-flash` defaults with `gemini-3.7-flash` for both
  Gemini audio transcription and structured analysis.
- Set the local ignored `.env` to explicit `AI_MODE=gemini`, rebuilt the API/worker
  images, and restarted the Compose services with the configured Gemini key.
- Added a configuration regression test. `go test ./...`, `go vet ./...`,
  `go build ./...`, and Compose startup completed successfully; no real audio call
  was submitted during verification.
- No Git commit or push was created.
