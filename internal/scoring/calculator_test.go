package scoring

import "testing"

func TestCalculateReturnsAllCriterionDetailsAndBackendOwnedTotal(t *testing.T) {
	scores := completeScores()

	result, err := Calculate(scores)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	if result.Total != 67 {
		t.Errorf("Total = %d, want 67", result.Total)
	}
	if result.Criteria["needs_discovery"].Feedback != "Asked about budget and timeline." {
		t.Errorf("needs_discovery feedback = %q, want criterion detail", result.Criteria["needs_discovery"].Feedback)
	}
}

func TestCalculateRejectsOutOfRangeCriterionScores(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		score int
	}{
		{name: "above maximum", key: "greeting", score: 6},
		{name: "negative", key: "greeting", score: -1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scores := completeScores()
			scores[test.key] = CriterionScore{Score: test.score, Feedback: "Invalid score."}
			if _, err := Calculate(scores); err == nil {
				t.Fatal("Calculate() error = nil, want invalid score rejection")
			}
		})
	}
}

func TestCalculateRejectsUnrecognizedExtraCriterionInsteadOfExceeding100(t *testing.T) {
	scores := map[string]CriterionScore{
		"greeting":           {Score: 5, Feedback: "OK"},
		"rapport":            {Score: 10, Feedback: "OK"},
		"needs_discovery":    {Score: 20, Feedback: "OK"},
		"presentation":       {Score: 15, Feedback: "OK"},
		"objection_handling": {Score: 20, Feedback: "OK"},
		"next_action":        {Score: 15, Feedback: "OK"},
		"communication":      {Score: 10, Feedback: "OK"},
		"closing":            {Score: 5, Feedback: "OK"},
		"model_extra":        {Score: 1, Feedback: "Must not be counted"},
	}

	if _, err := Calculate(scores); err == nil {
		t.Fatal("Calculate() error = nil, want unrecognized extra criterion rejection")
	}
}

func completeScores() map[string]CriterionScore {
	return map[string]CriterionScore{
		"greeting":           {Score: 4, Feedback: "Professional greeting."},
		"rapport":            {Score: 6, Feedback: "Some rapport."},
		"needs_discovery":    {Score: 12, Feedback: "Asked about budget and timeline."},
		"presentation":       {Score: 10, Feedback: "Clear presentation."},
		"objection_handling": {Score: 14, Feedback: "Addressed price concern."},
		"next_action":        {Score: 10, Feedback: "Scheduled follow-up."},
		"communication":      {Score: 7, Feedback: "Mostly clear."},
		"closing":            {Score: 4, Feedback: "Positive close."},
	}
}
