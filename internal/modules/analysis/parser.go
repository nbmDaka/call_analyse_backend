package analysis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"call_analyse_backend/internal/modules/scoring"
)

var requiredFields = []string{
	"summary",
	"needs",
	"objections",
	"refusal_reason",
	"mistakes",
	"strengths",
	"next_action",
	"criterion_results",
}

type analysisPayload struct {
	Summary            string                            `json:"summary"`
	DetectedLanguage   string                            `json:"detected_language,omitempty"`
	Needs              []string                          `json:"needs"`
	Objections         []string                          `json:"objections"`
	RefusalReason      *string                           `json:"refusal_reason"`
	Mistakes           []string                          `json:"mistakes"`
	Strengths          []string                          `json:"strengths"`
	NextAction         string                            `json:"next_action"`
	CriterionResults   map[string]scoring.CriterionScore `json:"criterion_results"`
	RoleMapping        *RoleMapping                      `json:"role_mapping,omitempty"`
	SpeechAnalytics    *SpeechAnalytics                  `json:"speech_analytics,omitempty"`
	Violations         []Violation                       `json:"violations,omitempty"`
	ActionableCoaching []string                          `json:"actionable_coaching,omitempty"`
	ModelTotal         json.RawMessage                   `json:"total_score,omitempty"`
}

// ParseAndValidate strictly decodes provider JSON into a complete Analysis using default scoring criteria.
func ParseAndValidate(raw []byte) (Analysis, error) {
	return ParseAndValidateWithCriteria(raw, scoring.Criteria())
}

// ParseAndValidateWithCriteria decodes provider JSON and validates against the specified criteria list.
func ParseAndValidateWithCriteria(raw []byte, criteriaList []scoring.Criterion) (Analysis, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Analysis{}, fmt.Errorf("decode analysis JSON: %w", err)
	}
	for _, field := range requiredFields {
		if _, ok := fields[field]; !ok {
			return Analysis{}, fmt.Errorf("analysis field %q is required", field)
		}
	}

	var payload analysisPayload
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return Analysis{}, fmt.Errorf("decode analysis JSON: %w", err)
	}
	if err := requireEOF(decoder); err != nil {
		return Analysis{}, err
	}

	if strings.TrimSpace(payload.Summary) == "" {
		return Analysis{}, fmt.Errorf("analysis summary is required")
	}
	if strings.TrimSpace(payload.NextAction) == "" {
		return Analysis{}, fmt.Errorf("analysis next_action is required")
	}
	if payload.Needs == nil || payload.Objections == nil || payload.Mistakes == nil || payload.Strengths == nil {
		return Analysis{}, fmt.Errorf("analysis list fields must be arrays")
	}
	if err := validateCriterionResultFields(fields["criterion_results"]); err != nil {
		return Analysis{}, err
	}
	if _, err := scoring.CalculateWithCriteria(payload.CriterionResults, criteriaList); err != nil {
		return Analysis{}, fmt.Errorf("validate criterion results: %w", err)
	}

	violations := payload.Violations
	if violations == nil {
		violations = []Violation{}
	}
	coaching := payload.ActionableCoaching
	if coaching == nil {
		coaching = []string{}
	}

	return Analysis{
		Summary:            payload.Summary,
		DetectedLanguage:   payload.DetectedLanguage,
		Needs:              payload.Needs,
		Objections:         payload.Objections,
		RefusalReason:      payload.RefusalReason,
		Mistakes:           payload.Mistakes,
		Strengths:          payload.Strengths,
		NextAction:         payload.NextAction,
		CriterionResults:   payload.CriterionResults,
		RoleMapping:        payload.RoleMapping,
		SpeechAnalytics:    payload.SpeechAnalytics,
		Violations:         violations,
		ActionableCoaching: coaching,
		RawJSON:            append(json.RawMessage(nil), raw...),
	}, nil
}

func requireEOF(decoder *json.Decoder) error {
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode analysis JSON: multiple JSON values")
		}
		return fmt.Errorf("decode analysis JSON: %w", err)
	}
	return nil
}

func validateCriterionResultFields(raw json.RawMessage) error {
	var results map[string]json.RawMessage
	if err := json.Unmarshal(raw, &results); err != nil {
		return fmt.Errorf("decode criterion_results: %w", err)
	}
	for key, result := range results {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(result, &fields); err != nil {
			return fmt.Errorf("decode criterion %q: %w", key, err)
		}
		if _, ok := fields["score"]; !ok {
			return fmt.Errorf("criterion %q score is required", key)
		}
		if _, ok := fields["feedback"]; !ok {
			return fmt.Errorf("criterion %q feedback is required", key)
		}
	}
	return nil
}
