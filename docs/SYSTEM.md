# System overview

AI Call Analysis (Callwise) is a Go backend platform for accepting sales-call audio, processing it
asynchronously, and presenting a transcript, structured speech & emotional analysis, rule-based
playbook compliance, grounded evidence quotes, actionable sales coaching, and backend-owned scoring.
The runtime has API, worker, and migration processes backed by PostgreSQL, Redis/Asynq, MinIO, and
Gemini 1.5 Flash (or deterministic fake providers).

The application is multi-tenant. Every ordinary account owns a personal workspace
and may also hold independent owner/admin/supervisor/manager memberships in company
workspaces. Calls, playbooks, golden standards, dashboard aggregates, object keys, and worker jobs carry an
explicit workspace ID. `users.platform_role` is limited to `user` and
`super_admin`; it is not a substitute for a workspace membership.

Tasks 1–9 provide the foundation: configuration, containers, migrations, auth/RBAC, MinIO storage,
transcripts, Gemini providers, Asynq queues, scoring, and multi-tenant workspaces.
Task 10 (Speech Intelligence & Playbooks) adds customizable workspace Playbooks, Golden Standards
cataloging, single-pass Gemini multimodal extraction (role mapping, talk-to-listen ratio, awkward
pauses >3.5s, interruptions, emotional tone), grounded evidence quotes, violations with severity badges,
and actionable coaching recommendations.

