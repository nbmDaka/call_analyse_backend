# API

All errors use `{ "error": { "code": "...", "message": "..." }, "request_id": "..." }`
and return `X-Request-ID`.

Health: `GET /health/live` and `GET /health/ready` (503 when PostgreSQL is unavailable).

Authentication:

- `POST /api/v1/auth/login` with `{email,password}` returns access and refresh tokens.
- `POST /api/v1/auth/refresh` rotates a refresh session.
- `POST /api/v1/auth/logout` revokes a refresh session.
- `GET /api/v1/me` returns the authenticated public user.

Calls require `Authorization: Bearer <access-token>`:

- `POST /api/v1/calls` accepts multipart field `file`, supports `.mp3`, `.wav`, `.m4a`,
  and returns `201` with a queued call.
- `GET /api/v1/calls?page=1&page_size=20` returns actor-scoped calls.
- `GET /api/v1/calls/{id}` returns call/audio/manager data and persisted transcript,
  analysis, and score when available.

`GET /api/v1/dashboard/summary` returns scoped total, completed, failed, and average-score
aggregates. Managers see their calls, supervisors their managers' calls, and admins all calls.

The companion frontend consumes these routes directly, normalizes the Go default
exported-field names in call responses, and does not calculate scores or enforce
permissions client-side.
