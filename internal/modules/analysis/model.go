// Package analysis defines structured call-analysis data shared by providers and workers.
package analysis

import (
	"encoding/json"

	"call_analyse_backend/internal/modules/scoring"
)

// CriterionResult is the provider's feedback for one backend-defined criterion.
type CriterionResult = scoring.CriterionScore

// Analysis is the validated structured analysis produced from a call transcript.
// RawJSON is retained only for audit/debug persistence; score totals always remain
// backend-owned by the scoring package.
type Analysis struct {
	Summary          string                     `json:"summary"`
	Needs            []string                   `json:"needs"`
	Objections       []string                   `json:"objections"`
	RefusalReason    *string                    `json:"refusal_reason"`
	Mistakes         []string                   `json:"mistakes"`
	Strengths        []string                   `json:"strengths"`
	NextAction       string                     `json:"next_action"`
	CriterionResults map[string]CriterionResult `json:"criterion_results"`
	RawJSON          json.RawMessage            `json:"-"`
}

// Result remains an alias for the Task 5 shared type while consumers migrate to
// the explicit validated Analysis name.
type Result = Analysis
