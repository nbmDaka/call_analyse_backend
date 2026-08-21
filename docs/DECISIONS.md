# Architecture decisions

## D-001: Use a modular Go monolith

The MVP uses `cmd/api`, `cmd/worker`, and `cmd/migrate` rather than microservices.
This keeps operational complexity low while separating long-running API and worker
concerns.

## D-002: Resolve `AI_MODE=auto` during configuration loading

Configuration resolves `auto` to Gemini when `GEMINI_API_KEY` is present and to the
deterministic fake provider only in development or test. Staging, production, and
unknown environments fail closed when auto lacks a Gemini key; production also
rejects fake mode.

## D-003: Wire the vertical slice before claiming runtime readiness

The API owns HTTP dependencies and starts only after PostgreSQL and MinIO are
available; the worker owns asynchronous processing; the migration binary is a
short-lived Compose prerequisite. Documentation advertises only implemented routes.

## D-004: Embed root SQL migrations and centralize score metadata

The root `migrations` package exports a Go embedded filesystem so the migration
command and future database tests use identical SQL. Eight named scoring criteria
are defined once in `internal/scoring`, with maxima totaling 100; the database schema
uses matching check constraints.

## D-005: Keep upload enqueue recoverable

The API enqueues a unique `process_call` task and durably transitions the call to
`queued` when the persistence boundary supports it. If enqueue fails after the
object/row are created, the call service compensates by deleting the metadata and
object. This is best-effort cleanup rather than a distributed transaction; the
worker remains idempotent for retries and duplicate deliveries.

## D-006: Separate platform identity from workspace authorization

JWTs carry only user identity and platform role. Workspace roles are loaded from
PostgreSQL for every workspace request so disabling or changing a membership takes
effect immediately. Platform superadmins use separate platform routes and receive no
implicit access to call content.

## D-007: Use one workspace abstraction for people and companies

Personal and company tenants share `workspaces` and `workspace_memberships`. Calls
carry required workspace, owner-user, and uploader-user IDs. The initial supervisor
hierarchy uses a same-workspace `supervisor_membership_id`, leaving room for teams.

## D-008: Preserve legacy data through additive migration

Migration 000003 creates personal tenants for existing users, creates a deterministic
legacy company tenant, preserves supervisor relationships, and backfills call
ownership before enforcing NOT NULL. Legacy role/manager columns and object keys are
retained temporarily; all new authorization uses memberships and workspace IDs.
