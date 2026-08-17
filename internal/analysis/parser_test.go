package analysis

import (
	"encoding/json"
	"testing"
)

func TestParseAndValidateAcceptsAllDefinedFieldsAndIgnoresModelTotal(t *testing.T) {
	payload := validAnalysisPayload()
	payload["total_score"] = 999
	raw := marshalAnalysisPayload(t, payload)

	result, err := ParseAndValidate(raw)
	if err != nil {
		t.Fatalf("ParseAndValidate() error = %v", err)
	}
	if result.Summary != "The client needs a budget-conscious proposal." {
		t.Errorf("Summary = %q, want parsed summary", result.Summary)
	}
	if result.NextAction != "Send a tailored proposal." {
		t.Errorf("NextAction = %q, want parsed next action", result.NextAction)
	}
	if len(result.CriterionResults) != 8 {
		t.Errorf("CriterionResults length = %d, want 8", len(result.CriterionResults))
	}
	if string(result.RawJSON) != string(raw) {
		t.Errorf("RawJSON = %s, want original payload for audit", result.RawJSON)
	}
}

func TestParseAndValidateRejectsInvalidAnalysisPayloads(t *testing.T) {
	tests := []struct {
		name    string
		payload func() []byte
	}{
		{
			name:    "malformed JSON",
			payload: func() []byte { return []byte(`{"summary":`) },
		},
		{
			name: "missing summary",
			payload: func() []byte {
				payload := validAnalysisPayload()
				delete(payload, "summary")
				return marshalAnalysisPayload(t, payload)
			},
		},
		{
			name: "missing needs",
			payload: func() []byte {
				payload := validAnalysisPayload()
				delete(payload, "needs")
				return marshalAnalysisPayload(t, payload)
			},
		},
		{
			name: "missing next action",
			payload: func() []byte {
				payload := validAnalysisPayload()
				delete(payload, "next_action")
				return marshalAnalysisPayload(t, payload)
			},
		},
		{
			name: "missing criterion",
			payload: func() []byte {
				payload := validAnalysisPayload()
				delete(payload["criterion_results"].(map[string]any), "closing")
				return marshalAnalysisPayload(t, payload)
			},
		},
		{
			name: "unknown criterion",
			payload: func() []byte {
				payload := validAnalysisPayload()
				payload["criterion_results"].(map[string]any)["unrecognized"] = map[string]any{"score": 1, "feedback": "Not allowed."}
				return marshalAnalysisPayload(t, payload)
			},
		},
		{
			name: "negative score",
			payload: func() []byte {
				payload := validAnalysisPayload()
				payload["criterion_results"].(map[string]any)["greeting"].(map[string]any)["score"] = -1
				return marshalAnalysisPayload(t, payload)
			},
		},
		{
			name: "score above criterion maximum",
			payload: func() []byte {
				payload := validAnalysisPayload()
				payload["criterion_results"].(map[string]any)["greeting"].(map[string]any)["score"] = 6
				return marshalAnalysisPayload(t, payload)
			},
		},
		{
			name: "unknown top-level field",
			payload: func() []byte {
				payload := validAnalysisPayload()
				payload["unexpected"] = true
				return marshalAnalysisPayload(t, payload)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseAndValidate(test.payload()); err == nil {
				t.Fatal("ParseAndValidate() error = nil, want invalid analysis rejection")
			}
		})
	}
}

func validAnalysisPayload() map[string]any {
	return map[string]any{
		"summary":        "The client needs a budget-conscious proposal.",
		"needs":          []string{"A proposal within budget"},
		"objections":     []string{"Price sensitivity"},
		"refusal_reason": nil,
		"mistakes":       []string{"Confirm the decision timeline."},
		"strengths":      []string{"Acknowledged the client need."},
		"next_action":    "Send a tailored proposal.",
		"criterion_results": map[string]any{
			"greeting":           map[string]any{"score": 5, "feedback": "Opened professionally."},
			"rapport":            map[string]any{"score": 10, "feedback": "Built rapport."},
			"needs_discovery":    map[string]any{"score": 20, "feedback": "Explored needs."},
			"presentation":       map[string]any{"score": 15, "feedback": "Presented clearly."},
			"objection_handling": map[string]any{"score": 20, "feedback": "Handled objections."},
			"next_action":        map[string]any{"score": 15, "feedback": "Agreed next action."},
			"communication":      map[string]any{"score": 10, "feedback": "Communicated clearly."},
			"closing":            map[string]any{"score": 5, "feedback": "Closed well."},
		},
	}
}

func marshalAnalysisPayload(t *testing.T, payload map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return raw
}
