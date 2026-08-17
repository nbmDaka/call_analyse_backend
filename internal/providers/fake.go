// Package providers contains adapters for external AI services and development fakes.
package providers

import (
	"context"

	"call_analyse_backend/internal/analysis"
	"call_analyse_backend/internal/scoring"
	"call_analyse_backend/internal/transcription"
)

// FakeAnalysisProvider provides deterministic analysis for development and tests.
type FakeAnalysisProvider struct{}

// Analyze returns fixed, complete structured analysis with every scoring criterion.
func (FakeAnalysisProvider) Analyze(_ context.Context, _ transcription.Transcript) (analysis.Analysis, error) {
	criteria := make(map[string]analysis.CriterionResult, len(scoring.Criteria()))
	for _, criterion := range scoring.Criteria() {
		criteria[criterion.Key] = analysis.CriterionResult{Score: criterion.Max / 2, Feedback: "Deterministic fake feedback."}
	}
	return analysis.Analysis{
		Summary:          "The client requested a budget-conscious proposal.",
		Needs:            []string{"A proposal within budget"},
		Objections:       []string{"Price sensitivity"},
		RefusalReason:    nil,
		Mistakes:         []string{"Confirm the decision timeline."},
		Strengths:        []string{"Acknowledged the client need."},
		NextAction:       "Send a tailored proposal.",
		CriterionResults: criteria,
	}, nil
}

var _ analysis.AnalysisProvider = FakeAnalysisProvider{}
