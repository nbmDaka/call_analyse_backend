# Task 6 analysis and scoring run

## Scope

Added strict analysis JSON validation and backend-owned score calculation.

## Changed

- `internal/analysis/model.go`
- `internal/analysis/parser.go`
- `internal/analysis/parser_test.go`
- `internal/analysis/provider.go`
- `internal/scoring/calculator.go`
- `internal/scoring/calculator_test.go`

## Verification

- `go test ./internal/analysis ./internal/scoring -v` — PASS.
- `go test ./...` — PASS.
- `go vet ./...` — PASS.
- `go build ./...` — PASS.

## Limitations

No real Gemini request or database persistence was run; those belong to later tasks.
No Git commit or push was created.
