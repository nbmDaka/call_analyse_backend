package scoring

import "fmt"

// CriterionScore is one criterion's model feedback and numeric score.
type CriterionScore struct {
	Score    int    `json:"score"`
	Feedback string `json:"feedback"`
}

// Score contains backend-validated criterion details and their backend-owned total.
type Score struct {
	Criteria map[string]CriterionScore `json:"criteria"`
	Total    int                       `json:"total"`
}

// Calculate validates the complete fixed scoring contract and calculates a total
// from criterion maxima defined by Criteria. It never accepts a caller total.
func Calculate(scores map[string]CriterionScore) (Score, error) {
	for key, score := range scores {
		criterion, ok := criteriaByKey[key]
		if !ok {
			return Score{}, fmt.Errorf("unknown score criterion %q", key)
		}
		if score.Score < 0 || score.Score > criterion.Max {
			return Score{}, fmt.Errorf("score for %q must be between 0 and %d", key, criterion.Max)
		}
	}

	total := 0
	validated := make(map[string]CriterionScore, len(criteria))
	for _, criterion := range Criteria() {
		score, ok := scores[criterion.Key]
		if !ok {
			return Score{}, fmt.Errorf("score for required criterion %q is missing", criterion.Key)
		}
		validated[criterion.Key] = score
		total += score.Score
	}
	if total > 100 {
		return Score{}, fmt.Errorf("total score must not exceed 100")
	}

	return Score{Criteria: validated, Total: total}, nil
}
