// Package scoring owns the criteria and validation for call-analysis scores.
package scoring

import "fmt"

const (
	CriterionGreeting          = "greeting"
	CriterionRapport           = "rapport"
	CriterionNeedsDiscovery    = "needs_discovery"
	CriterionPresentation      = "presentation"
	CriterionObjectionHandling = "objection_handling"
	CriterionNextAction        = "next_action"
	CriterionCommunication     = "communication"
	CriterionClosing           = "closing"
)

// Criterion defines a score dimension and its inclusive maximum.
type Criterion struct {
	Key string
	Max int
}

var criteria = []Criterion{
	{Key: CriterionGreeting, Max: 5},
	{Key: CriterionRapport, Max: 10},
	{Key: CriterionNeedsDiscovery, Max: 20},
	{Key: CriterionPresentation, Max: 15},
	{Key: CriterionObjectionHandling, Max: 20},
	{Key: CriterionNextAction, Max: 15},
	{Key: CriterionCommunication, Max: 10},
	{Key: CriterionClosing, Max: 5},
}

var criteriaByKey = func() map[string]Criterion {
	byKey := make(map[string]Criterion, len(criteria))
	for _, criterion := range criteria {
		byKey[criterion.Key] = criterion
	}
	return byKey
}()

// Criteria returns the complete scoring contract in its stable display order.
func Criteria() []Criterion {
	return append([]Criterion(nil), criteria...)
}

// CalculateTotal validates known criterion scores and returns their total.
func CalculateTotal(scores map[string]int) (int, error) {
	total := 0
	for key, score := range scores {
		criterion, ok := criteriaByKey[key]
		if !ok {
			return 0, fmt.Errorf("unknown score criterion %q", key)
		}
		if score < 0 || score > criterion.Max {
			return 0, fmt.Errorf("score for %q must be between 0 and %d", key, criterion.Max)
		}
		total += score
	}
	if total > 100 {
		return 0, fmt.Errorf("total score must not exceed 100")
	}
	return total, nil
}
