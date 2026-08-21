# API

All errors use `{ "error": { "code": "...", "message": "..." }, "request_id": "..." }`
and return `X-Request-ID`.

Health: `GET /health/live` and `GET /health/ready` (503 when PostgreSQL is unavailable).

Authentication:

- `POST /api/v1/auth/login` with `{email,password}` returns access and refresh tokens.
- `POST /api/v1/auth/refresh` rotates a refresh session.
- `POST /api/v1/auth/logout` revokes a refresh session.
- `GET /api/v1/me` returns the authenticated public user.

`GET /api/v1/me` includes `platform_role` and user status. JWTs identify the user
and platform role only; they never act as the source of truth for workspace roles.

Workspace routes require `Authorization: Bearer <access-token>` and reload the
current membership from PostgreSQL:

- `GET|POST /api/v1/workspaces`
- `GET|PATCH /api/v1/workspaces/{workspaceID}`
- `GET|POST /api/v1/workspaces/{workspaceID}/members`
- `PATCH|DELETE /api/v1/workspaces/{workspaceID}/members/{membershipID}`
- `GET|POST /api/v1/workspaces/{workspaceID}/calls`
- `GET /api/v1/workspaces/{workspaceID}/calls/{callID}`
- `GET /api/v1/workspaces/{workspaceID}/dashboard`
- `GET|POST /api/v1/workspaces/{workspaceID}/playbooks`
- `GET|PATCH|DELETE /api/v1/workspaces/{workspaceID}/playbooks/{playbookID}`
- `GET|POST /api/v1/workspaces/{workspaceID}/golden-standards`
- `DELETE /api/v1/workspaces/{workspaceID}/golden-standards/{id}`

`GET /api/v1/workspaces/{workspaceID}/calls/{callID}` returns:
- Standard call metadata (status, manager, audio object key).
- Transcripts with segments, timestamps, and speaker labels.
- `role_mapping` (`manager_speaker`, `client_speaker`).
- `speech_analytics` (talk-to-listen %, awkward pauses >3.5s, interruptions, emotional tone).
- `criterion_results` (playbook criteria results with scores, max_scores, passed, feedback, evidence_quote, timestamp).
- `violations` (severity: high/medium/low, title, timestamp, quote, fix_advice).
- `actionable_coaching` (tactical recommendations for the sales rep).
- `recommended_golden_standards` (relevant stellar call excerpts).

Managers see their own calls; supervisors see their own calls and assigned active
managers; workspace owner/admin sees all calls in that workspace. Suspended workspaces
remain readable but reject uploads. Disabled memberships are rejected immediately,
and cross-tenant resource access returns 404.


Platform-superadmin routes are separate:

- `GET|POST /api/v1/platform/workspaces`
- `GET /api/v1/platform/users`
- `GET /api/v1/platform/metrics`
- `PATCH /api/v1/platform/workspaces/{workspaceID}/status`
- `PATCH /api/v1/platform/users/{userID}/status`

Platform mutations write sanitized audit events. These endpoints expose system
metadata and aggregates, not transcript/audio content, and no impersonation flow exists.

The old `/api/v1/calls` and `/api/v1/dashboard/summary` patterns remain registered as
deprecated compatibility routes, but the production tenant store requires explicit
workspace scope. The frontend uses only the new workspace routes.

The companion frontend consumes these routes directly, normalizes the Go default
exported-field names in call responses, and does not calculate scores or enforce
permissions client-side.

For separate local frontend development, `CORS_ALLOWED_ORIGINS` configures the
allowlist for browser origins such as `http://localhost:5173` and
`http://127.0.0.1:5173`. Preflight requests allow `Authorization` and `Content-Type`.
