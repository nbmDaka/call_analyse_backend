package scoring

import (
	"reflect"
	"testing"
)

func TestCriteriaMatchCallAnalysisContract(t *testing.T) {
	want := []Criterion{
		{Key: "greeting", Max: 5},
		{Key: "rapport", Max: 10},
		{Key: "needs_discovery", Max: 20},
		{Key: "presentation", Max: 15},
		{Key: "objection_handling", Max: 20},
		{Key: "next_action", Max: 15},
		{Key: "communication", Max: 10},
		{Key: "closing", Max: 5},
	}

	if got := Criteria(); !reflect.DeepEqual(got, want) {
		t.Errorf("Criteria() = %#v, want %#v", got, want)
	}
}

func TestCalculateTotalAcceptsEveryCriterionMaximum(t *testing.T) {
	scores := map[string]int{
		"greeting":           5,
		"rapport":            10,
		"needs_discovery":    20,
		"presentation":       15,
		"objection_handling": 20,
		"next_action":        15,
		"communication":      10,
		"closing":            5,
	}

	total, err := CalculateTotal(scores)
	if err != nil {
		t.Fatalf("CalculateTotal() error = %v", err)
	}
	if total != 100 {
		t.Errorf("CalculateTotal() = %d, want 100", total)
	}
}

func TestCalculateTotalRejectsScoreAboveCriterionMaximum(t *testing.T) {
	_, err := CalculateTotal(map[string]int{"greeting": 6})
	if err == nil {
		t.Fatal("CalculateTotal() error = nil, want error for score above criterion maximum")
	}
}

func TestCalculateTotalRejectsUnknownCriterion(t *testing.T) {
	_, err := CalculateTotal(map[string]int{"unknown": 1})
	if err == nil {
		t.Fatal("CalculateTotal() error = nil, want error for unknown criterion")
	}
}
