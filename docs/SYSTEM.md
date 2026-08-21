# System overview

AI Call Analysis is a Go backend MVP for accepting sales-call audio, processing it
asynchronously, and presenting a transcript, structured analysis, and backend-owned
score. The target runtime has API, worker, and migration processes backed by
PostgreSQL, Redis/Asynq, MinIO, and Gemini or deterministic fake providers.

The application is multi-tenant. Every ordinary account owns a personal workspace
and may also hold independent owner/admin/supervisor/manager memberships in company
workspaces. Calls, dashboard aggregates, object keys, and worker jobs carry an
explicit workspace ID. `users.platform_role` is limited to `user` and
`super_admin`; it is not a substitute for a workspace membership.

Tasks 1–4 provide configuration, container scaffolding, embedded SQL migrations,
PostgreSQL pool lifecycle, call status transitions, scoring criteria, authentication
primitives/RBAC, MinIO storage boundary, upload validation, and scoped call services.
Task 5 adds transcript/provider/queue contracts; Task 6 adds strict analysis
validation and backend scoring; Task 7 adds worker orchestration and idempotency;
Task 8 adds HTTP routes, result read models, dashboard, and API runtime wiring;
Task 9 adds Compose migration ordering and local runtime documentation.
