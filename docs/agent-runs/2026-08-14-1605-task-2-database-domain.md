# Task 2 database lifecycle and domain primitives

## Scope

Implemented the initial PostgreSQL schema, embedded migration assets, pgx pool
lifecycle, migration command wiring, call status transitions, and scoring metadata.

## TDD evidence

RED command:

```text
go test ./internal/calls ./internal/scoring -v
```

Result: exit 1 because `Status`, transition functions, scoring criterion constants,
and `CalculateTotal` were undefined.

GREEN command:

```text
go test ./internal/calls ./internal/scoring -v
```

Result: exit 0. The valid lifecycle, failure transitions, invalid completed-state
transition, all criterion maxima, the total of 100, and invalid scoring inputs passed.

## Verification

```text
go test ./...
go build ./...
go vet ./...
docker compose config
```

All exited 0 after supplying the required temporary development environment values
to Compose. `go test ./...` initially reported a missing `pgxpool` transitive checksum;
`go mod tidy` recorded the required `github.com/jackc/puddle/v2` checksum, and the
fresh retry passed.

The migration command was also run with a PostgreSQL URL at unavailable
`127.0.0.1:1`. It emitted `migration database connection failed` and exited 1 as
required.

`git diff --check` reported pre-existing trailing whitespace only in the Task 1 plan
and design documents. Those unrelated files were deliberately preserved. A scoped
Task 2 diff check emitted no whitespace diagnostics.

## Limitations

No Docker runtime stack, live PostgreSQL migration, or real Gemini request was run.
Those are outside this task's requested verification.

## Commits

None, per user instruction.
