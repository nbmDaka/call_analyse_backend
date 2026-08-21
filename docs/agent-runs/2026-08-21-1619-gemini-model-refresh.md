# Gemini model refresh

Date: 2026-08-21 16:19 Asia/Qyzylorda

## Scope

Restore the Gemini-backed call pipeline by replacing the retired Gemini model
configured for both transcription and analysis.

## Changes

- Added a configuration regression test for the current default Gemini model.
- Changed the backend and Compose defaults for `GEMINI_TRANSCRIPTION_MODEL` and
  `GEMINI_ANALYSIS_MODEL` to `gemini-3.7-flash`.
- Updated the local ignored `.env` to use explicit `AI_MODE=gemini` and the same
  model for transcription and analysis.
- Updated current-state documentation.

## Verification

- RED: the new config test failed with the old `gemini-2.0-flash` defaults.
- GREEN: `go test ./...` passed.
- `go vet ./...` passed.
- `go build ./...` passed.
- `docker compose --env-file .env up -d --build api worker` completed successfully.
- Compose configuration resolved `AI_MODE=gemini` and `gemini-3.7-flash` for both
  model settings; the worker started and joined the `calls` queue.
- No real audio was submitted to Gemini, so end-to-end provider execution remains
  unverified in this session.
