# Final full-stack review run

Date: 2026-08-17

## Findings

- The original frontend refresh implementation could issue parallel refresh-token
  rotations after concurrent 401 responses. This was a real session stability defect.
- Separate frontend/backend development had no Vite proxy or backend CORS handling,
  so browser login/upload would be blocked by cross-origin policy.
- Supervisor upload controls were shown despite backend rejecting supervisor uploads.
- Login returned to an unvalidated navigation state path, which was unsafe with the
  current React Router advisory.

## Fixes

- Deduplicated frontend refresh calls with a shared in-flight promise and added a
  concurrent-401 regression test.
- Added `CORS_ALLOWED_ORIGINS` configuration, safe local defaults, CORS middleware,
  preflight behavior, tests, Compose wiring, and API documentation.
- Added frontend role-aware upload controls and navigation guard.
- Restricted post-login redirects to safe internal paths and retained the advisory as
  non-blocking because the affected dependency is not upgraded in this review.

## Verification

- Backend `go test -count=1 ./...`, `go vet ./...`, and `go build ./...` — passed.
- Frontend `npm test` (5 tests), `npm run typecheck`, and `npm run build` — passed.
- `npm audit --omit=dev --audit-level=high` — passed; full audit retains non-blocking
  dev-tool advisories and a moderate React Router advisory.
- `docker compose --env-file .env.example config` — passed.
- Browser tooling was unavailable in this environment.
- Docker runtime smoke remains unavailable; Compose config is verified separately.
