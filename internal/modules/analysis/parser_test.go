package analysis

import (
	"encoding/json"
	"testing"

	"call_analyse_backend/internal/modules/scoring"
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

func TestParseAndValidateAcceptsEnrichedSpeechAnalyticsAndViolations(t *testing.T) {
	payload := validAnalysisPayload()
	payload["role_mapping"] = map[string]any{
		"manager_speaker": "Speaker 0",
		"client_speaker":  "Speaker 1",
	}
	payload["speech_analytics"] = map[string]any{
		"talk_to_listen": map[string]any{
			"manager_percentage": 58,
			"client_percentage":  42,
		},
		"awkward_pauses": []map[string]any{
			{"start_seconds": 12.0, "end_seconds": 16.5, "duration_seconds": 4.5},
		},
		"interruptions": []map[string]any{
			{"timestamp_seconds": 45.2, "interrupted_by": "manager", "context": "Interrupted during pricing question"},
		},
		"emotional_tone": map[string]any{
			"manager_tone":    "confident",
			"client_tone":     "skeptical",
			"sentiment_shift": "positive",
		},
	}
	payload["violations"] = []map[string]any{
		{
			"severity":          "high",
			"title":             "Premature price drop",
			"quote":             "We can offer 20% discount right away",
			"timestamp_seconds": 50.5,
			"fix_advice":        "Explain value before discounting",
		},
	}
	payload["actionable_coaching"] = []string{
		"Pause 2 seconds before answering objections",
	}
	raw := marshalAnalysisPayload(t, payload)

	result, err := ParseAndValidate(raw)
	if err != nil {
		t.Fatalf("ParseAndValidate() error = %v", err)
	}
	if result.RoleMapping == nil || result.RoleMapping.ManagerSpeaker != "Speaker 0" {
		t.Errorf("RoleMapping = %+v, want manager Speaker 0", result.RoleMapping)
	}
	if result.SpeechAnalytics == nil || result.SpeechAnalytics.TalkToListen.ManagerPercentage != 58 {
		t.Errorf("TalkToListen = %+v, want 58%% manager", result.SpeechAnalytics)
	}
	if len(result.Violations) != 1 || result.Violations[0].Severity != "high" {
		t.Errorf("Violations = %+v, want 1 high severity violation", result.Violations)
	}
	if len(result.ActionableCoaching) != 1 {
		t.Errorf("ActionableCoaching = %+v, want 1 recommendation", result.ActionableCoaching)
	}
}

func TestParseAndValidateAcceptsDetectedLanguageAndCustomCriteria(t *testing.T) {
	customCriteria := []scoring.Criterion{
		{Key: "custom_kpi", Max: 50},
		{Key: "custom_closure", Max: 50},
	}
	payload := map[string]any{
		"summary":           "Қоңырау қазақ тілінде өтті.",
		"detected_language": "kk",
		"needs":             []string{"Жеке ұсыныс"},
		"objections":        []string{"Бағасы жоғары"},
		"refusal_reason":    nil,
		"mistakes":          []string{"Сұрақтар аз қойылды"},
		"strengths":         []string{"Жақсы байланыс"},
		"next_action":       "Келесі аптада хабарласу",
		"criterion_results": map[string]any{
			"custom_kpi":     map[string]any{"score": 40, "feedback": "Жақсы орындалды"},
			"custom_closure": map[string]any{"score": 45, "feedback": "Сәтті аяқталды"},
		},
	}
	raw := marshalAnalysisPayload(t, payload)

	result, err := ParseAndValidateWithCriteria(raw, customCriteria)
	if err != nil {
		t.Fatalf("ParseAndValidateWithCriteria() error = %v", err)
	}
	if result.DetectedLanguage != "kk" {
		t.Errorf("DetectedLanguage = %q, want kk", result.DetectedLanguage)
	}
	if len(result.CriterionResults) != 2 {
		t.Errorf("CriterionResults len = %d, want 2", len(result.CriterionResults))
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
