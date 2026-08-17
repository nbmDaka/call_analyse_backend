# Agent instructions

Before changing this repository, read `docs/SYSTEM.md`, `docs/ARCHITECTURE.md`,
`docs/CURRENT_STATE.md`, `docs/DECISIONS.md`, and `docs/AGENT_LOG.md`.

- Work only within this backend repository unless the user explicitly expands scope.
- Follow red-green-refactor for production behavior. Add a failing behavioral test,
  observe its failure, implement the smallest change, and re-run relevant tests.
- Do not commit or push unless the user explicitly requests it.
- Never log or return passwords, JWTs, refresh tokens, API keys, or full transcripts.
- Keep external systems behind focused package boundaries and use request/job contexts
  with explicit provider timeouts.
- `AI_MODE=auto` may resolve to fake only in development or test when no Gemini key
  is configured. All other environments reject auto without `GEMINI_API_KEY`, and
  production always rejects `AI_MODE=fake`.
- Update affected documentation, append `docs/AGENT_LOG.md`, and create a factual
  `docs/agent-runs/YYYY-MM-DD-HHMM-<slug>.md` record after each implementation session.
- Run and record the applicable tests, build, vet, Compose, and diff checks. State
  only outcomes supported by fresh command output.
