# Task 2 fix round 1: scoring contract

## Change

Replaced the obsolete scoring contract with the exact eight entries required for
call analysis: `greeting` 5, `rapport` 10, `needs_discovery` 20, `presentation` 15,
`objection_handling` 20, `next_action` 15, `communication` 10, and `closing` 5.
The matching initial-schema score columns and checks were updated at the same time.

## TDD evidence

RED:

```text
go test ./internal/scoring -run 'TestCriteriaMatchCallAnalysisContract|TestCalculateTotalAcceptsEveryCriterionMaximum|TestCalculateTotalRejectsScoreAboveCriterionMaximum' -v
```

Result: exit 1 because the old contract lacked `rapport`, retained obsolete entries,
and accepted a greeting score of 6.

GREEN:

```text
gofmt -w internal\scoring\model.go internal\scoring\model_test.go
go test ./internal/scoring -run 'TestCriteriaMatchCallAnalysisContract|TestCalculateTotalAcceptsEveryCriterionMaximum|TestCalculateTotalRejectsScoreAboveCriterionMaximum' -v
```

Result: exit 0, with all focused tests passing.

## Verification

```text
go test ./...
go build ./...
go vet ./...
```

Each command exited 0. A focused source scan confirmed that obsolete scoring names
are absent and the new identifiers occur in both the Go model and SQL migration.

## Commits

None, per user instruction.
