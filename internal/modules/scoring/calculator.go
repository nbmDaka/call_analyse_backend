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
	return CalculateWithCriteria(scores, Criteria())
}

// CalculateWithCriteria validates criterion scores against any slice of criteria.
func CalculateWithCriteria(scores map[string]CriterionScore, criteriaList []Criterion) (Score, error) {
	if len(criteriaList) == 0 {
		criteriaList = Criteria()
	}

	byKey := make(map[string]Criterion, len(criteriaList))
	totalMax := 0
	for _, c := range criteriaList {
		byKey[c.Key] = c
		totalMax += c.Max
	}

	for key, score := range scores {
		criterion, ok := byKey[key]
		if !ok {
			return Score{}, fmt.Errorf("unknown score criterion %q", key)
		}
		if score.Score < 0 || score.Score > criterion.Max {
			return Score{}, fmt.Errorf("score for %q must be between 0 and %d", key, criterion.Max)
		}
	}

	total := 0
	validated := make(map[string]CriterionScore, len(criteriaList))
	for _, criterion := range criteriaList {
		score, ok := scores[criterion.Key]
		if !ok {
			return Score{}, fmt.Errorf("score for required criterion %q is missing", criterion.Key)
		}
		validated[criterion.Key] = score
		total += score.Score
	}
	if total > totalMax || (totalMax <= 100 && total > 100) {
		return Score{}, fmt.Errorf("total score must not exceed maximum possible score")
	}

	return Score{Criteria: validated, Total: total}, nil
}
