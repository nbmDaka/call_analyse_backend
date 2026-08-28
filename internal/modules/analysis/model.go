// Package analysis defines structured call-analysis data shared by providers and workers.
package analysis

import (
	"encoding/json"

	"call_analyse_backend/internal/modules/scoring"
)

// CriterionResult is the provider's feedback for one backend-defined criterion.
type CriterionResult = scoring.CriterionScore

// RoleMapping identifies speaker labels in a call transcript.
type RoleMapping struct {
	ManagerSpeaker string `json:"manager_speaker"`
	ClientSpeaker  string `json:"client_speaker"`
}

// TalkToListenRatio represents the proportion of speech time between manager and client.
type TalkToListenRatio struct {
	ManagerPercentage float64 `json:"manager_percentage"`
	ClientPercentage  float64 `json:"client_percentage"`
}

// AwkwardPause records dead air or prolonged hesitation in seconds.
type AwkwardPause struct {
	StartSeconds    float64 `json:"start_seconds"`
	EndSeconds      float64 `json:"end_seconds"`
	DurationSeconds float64 `json:"duration_seconds"`
}

// Interruption captures an instance where one party spoke over another.
type Interruption struct {
	TimestampSeconds float64 `json:"timestamp_seconds"`
	InterruptedBy    string  `json:"interrupted_by"`
	Context          string  `json:"context"`
}

// EmotionalTone captures vocal characteristics and sentiment shifts during the call.
type EmotionalTone struct {
	ManagerTone    string `json:"manager_tone"`
	ClientTone     string `json:"client_tone"`
	SentimentShift string `json:"sentiment_shift"`
}

// SpeechAnalytics aggregates acoustic and conversational dynamics.
type SpeechAnalytics struct {
	TalkToListen   *TalkToListenRatio `json:"talk_to_listen,omitempty"`
	AwkwardPauses  []AwkwardPause     `json:"awkward_pauses"`
	Interruptions  []Interruption     `json:"interruptions"`
	EmotionalTone  *EmotionalTone     `json:"emotional_tone,omitempty"`
}

// Violation records a critical or moderate sales error with grounded advice.
type Violation struct {
	Severity         string   `json:"severity"` // "low", "medium", "high"
	Title            string   `json:"title"`
	Quote            string   `json:"quote"`
	TimestampSeconds *float64 `json:"timestamp_seconds,omitempty"`
	FixAdvice        string   `json:"fix_advice"`
}

// Analysis is the validated structured analysis produced from a call transcript or audio.
type Analysis struct {
	Summary            string                     `json:"summary"`
	DetectedLanguage   string                     `json:"detected_language,omitempty"`
	Needs              []string                   `json:"needs"`
	Objections         []string                   `json:"objections"`
	RefusalReason      *string                    `json:"refusal_reason"`
	Mistakes           []string                   `json:"mistakes"`
	Strengths          []string                   `json:"strengths"`
	NextAction         string                     `json:"next_action"`
	CriterionResults   map[string]CriterionResult `json:"criterion_results"`
	RoleMapping        *RoleMapping               `json:"role_mapping,omitempty"`
	SpeechAnalytics    *SpeechAnalytics           `json:"speech_analytics,omitempty"`
	Violations         []Violation                `json:"violations"`
	ActionableCoaching []string                   `json:"actionable_coaching"`
	RawJSON            json.RawMessage            `json:"-"`
}

// Result remains an alias for the shared type while consumers migrate.
type Result = Analysis
