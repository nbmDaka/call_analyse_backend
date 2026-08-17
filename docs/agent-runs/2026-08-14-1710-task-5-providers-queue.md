# Task 5: Providers and queue contracts

## Scope

Implemented Task 5 in `call_analyse_backend`: transcript types, deterministic fake
providers, Gemini HTTP adapters, and Asynq process-call task definitions.

## TDD record

- RED: Added transcription/queue/provider contract tests, then ran
  `go test ./internal/transcription ./internal/queue -v` and
  `go test ./internal/providers -v`. The new packages failed to build because the
  transcript symbols, task constructor/payload, and provider implementations did not
  yet exist.
- GREEN: Added the minimal provider and queue implementations, then ran
  `go test ./internal/transcription ./internal/queue -v` and
  `go test ./internal/providers -v`. All focused tests passed.

## Implementation

- `internal/transcription` owns audio input, transcript/segment types, speaker labels,
  provider boundary, and deterministic fake transcript data. Unavailable timestamps
  are represented by nil pointers.
- `internal/analysis` supplies the minimal shared result/provider contract required by
  Task 5's fake and Gemini analysis implementations. Strict parsing and scoring remain
  Task 6 responsibilities.
- `internal/providers` uses the Gemini `generateContent` HTTP endpoint with a
  configured model, API-key header, inline base64 audio, explicit HTTP timeout,
  structured JSON analysis request, response extraction, and body-free errors.
- `internal/queue` creates a UUID-validated `process_call` Asynq task. Its stable
  JSON payload and fixed queue/type supply a call-ID-derived uniqueness key, with a
  bounded retry count.

## Verification

- Passed: `go test ./internal/transcription ./internal/queue -v`
- Passed: `go test ./internal/providers -v`
- Passed: `go test ./...`
- Passed: `go vet ./...`
- Passed: `go build ./...`
- Pending final recorded result: `docker compose --env-file .env.example config`
- Known unrelated check failure: `git diff --check` reports trailing whitespace in
  pre-existing `docs/superpowers/plans/2026-08-14-ai-call-analysis-mvp-plan.md` and
  `docs/superpowers/specs/2026-08-14-ai-call-analysis-mvp-design.md`.

No real Gemini request or Docker runtime smoke test was performed.

Commits created: none
