# Architecture

The MVP is a modular monolith with focused package boundaries and three executable
processes. API and worker share PostgreSQL, MinIO, Redis/Asynq, and provider boundaries;
migrations run as a short-lived Compose dependency.

```text
Client
  |
  v
API ---- PostgreSQL (Workspaces, Memberships, Playbooks, Golden Standards, Calls, Analyses)
 |  \---- MinIO (Audio Storage)
 \----- Redis/Asynq ---- Worker ---- MinIO
                           |
                           v
                Gemini 1.5 Flash (Raw Audio + Playbook)
                or deterministic fake provider
```

`internal/config`, `internal/platform/database`, `internal/modules/calls`, `internal/modules/scoring`,
`internal/modules/auth`, `internal/modules/users`, `internal/transport/http/middleware`,
`internal/platform/storage`, `internal/platform/queue`, `internal/modules/transcription`,
`internal/modules/analysis`, `internal/modules/playbooks`, `internal/modules/golden_standards`,
`internal/jobs`, `internal/modules/dashboard`, and `internal/transport/http` are wired.
API startup wires the real services and gracefully shuts down; worker startup owns asynchronous
processing and persisted checkpoints.

## Audio and Speech Analysis Pipeline

1. **Audio Ingestion**: Audio files (.mp3/.wav) are validated and stored in MinIO.
2. **Channel & Speaker Handling**:
   - Stereo audio: Direct channel separation (Left = Manager, Right = Client).
   - Mono audio: Speaker diarization (Speaker 0 / Speaker 1) + LLM role mapping in prompt.
3. **Structured Single-Pass Extraction (Gemini 1.5 Flash)**:
   - Full transcript with word/segment timestamps.
   - Speech Metrics: Talk-to-Listen ratio (%), awkward pauses (>3.5s), interruptions, emotional tone.
   - Playbook Compliance: Criterion-by-criterion check with grounded evidence quotes and timestamps.
   - Violations & Coaching: Categorized violations with severity levels (high/medium/low) and tactical recommendations.
4. **Scoring & Persistence**:
   - Backend calculates score based on workspace Playbook criteria and penalty weights.
   - Persists structured analysis and score into PostgreSQL (`call_analyses`, `call_scores`).
   - Links matching Golden Standard examples for coaching.

Tenant authorization is rebuilt for every `/workspaces/{workspaceID}/...` request
by joining the authenticated user to the current workspace and membership rows.
The resulting actor is passed through handlers, services, and stores. Call list,
detail, pagination, and dashboard SQL apply `workspace_id` before role scope,
pagination, or aggregation. Supervisor scope uses same-workspace membership IDs.

`internal/modules/workspaces`, `memberships`, `playbooks`, `golden_standards`, and `platform`
own tenant lifecycle, membership management, scoring rules, and platform administration.
Platform mutations are separate from workspace endpoints and write sanitized `audit_events`.
New Asynq tasks contain both workspace and call IDs; the Gemini provider remains unaware of authorization.

The companion `call_analyse_frontend` repository is a React/TypeScript client using
the REST API, TanStack Query for server state, and React state for local UI state.
