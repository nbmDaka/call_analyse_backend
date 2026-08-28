package providers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"call_analyse_backend/internal/modules/analysis"
	"call_analyse_backend/internal/modules/scoring"
	"call_analyse_backend/internal/modules/transcription"
)

func TestFakeAnalysisProviderReturnsRequiredStableFieldsAndCriteria(t *testing.T) {
	provider := FakeAnalysisProvider{}
	transcript := transcription.TranscriptResult{Text: "manager: Hello\nclient: I need a proposal"}

	first, err := provider.Analyze(context.Background(), transcript)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	second, err := provider.Analyze(context.Background(), transcript)
	if err != nil {
		t.Fatalf("Analyze() second error = %v", err)
	}

	if first.Summary == "" || first.NextAction == "" || len(first.Needs) == 0 || len(first.Objections) == 0 || len(first.Mistakes) == 0 || len(first.Strengths) == 0 {
		t.Errorf("Analyze() = %#v, want every required analysis field", first)
	}
	if first.Summary != second.Summary || first.NextAction != second.NextAction {
		t.Errorf("Analyze() is not deterministic: %#v then %#v", first, second)
	}
	for _, criterion := range scoring.Criteria() {
		result, ok := first.CriterionResults[criterion.Key]
		if !ok {
			t.Errorf("criterion_results missing %q", criterion.Key)
			continue
		}
		if result.Score < 0 || result.Score > criterion.Max {
			t.Errorf("criterion %q score = %d, want 0..%d", criterion.Key, result.Score, criterion.Max)
		}
	}
}

func TestGeminiTranscribeSendsConfiguredModelAPIKeyAndInlineAudio(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/transcription-model:generateContent" {
			t.Errorf("path = %q, want configured transcription model", r.URL.Path)
		}
		if got := r.Header.Get("x-goog-api-key"); got != "test-api-key" {
			t.Errorf("x-goog-api-key = %q, want API key", got)
		}
		if strings.Contains(r.URL.RawQuery, "test-api-key") {
			t.Error("API key must not be placed in the request URL")
		}

		var request geminiGenerateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		part := request.Contents[0].Parts[1]
		if part.InlineData == nil || part.InlineData.MIMEType != "audio/wav" {
			t.Fatalf("inline audio = %#v, want MIME type audio/wav", part.InlineData)
		}
		if part.InlineData.Data != base64.StdEncoding.EncodeToString([]byte("audio-bytes")) {
			t.Errorf("inline audio data = %q, want base64 audio bytes", part.InlineData.Data)
		}
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"manager: Hello"}]}}]}`))
	}))
	defer server.Close()

	provider := newTestGemini(t, server.URL, time.Second)
	result, err := provider.Transcribe(context.Background(), transcription.AudioInput{
		Filename: "call.wav", MIMEType: "audio/wav", Data: []byte("audio-bytes"),
	})
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if result.Text != "manager: Hello" {
		t.Errorf("Transcribe() text = %q, want response text", result.Text)
	}
}

func TestGeminiAnalyzeUsesConfiguredModelAndExtractsJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/analysis-model:generateContent" {
			t.Errorf("path = %q, want configured analysis model", r.URL.Path)
		}
		var request geminiGenerateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.GenerationConfig == nil || request.GenerationConfig.ResponseMIMEType != "application/json" {
			t.Errorf("generation config = %#v, want JSON response request", request.GenerationConfig)
		}
		if !strings.Contains(request.Contents[0].Parts[0].Text, "transcript text") {
			t.Error("analysis prompt does not contain transcript text")
		}
		writeGeminiText(t, w, completeGeminiAnalysisJSON(t))
	}))
	defer server.Close()

	provider := newTestGemini(t, server.URL, time.Second)
	result, err := provider.Analyze(context.Background(), transcription.TranscriptResult{Text: "transcript text"})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Summary != "Summary" || result.NextAction != "Send quote" || result.CriterionResults["greeting"].Score != 5 {
		t.Errorf("Analyze() = %#v, want extracted structured analysis", result)
	}
}

func TestGeminiAnalyzeWithCustomCriteriaAndLanguage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request geminiGenerateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		promptText := request.Contents[0].Parts[0].Text
		if !strings.Contains(promptText, "custom_intro") || !strings.Contains(promptText, "Вводная часть") {
			t.Error("prompt does not contain dynamic criterion custom_intro")
		}
		customJSON := map[string]any{
			"summary":           "Қоңырау қазақша өтті.",
			"detected_language": "kk",
			"needs":             []string{"қажеттілік"},
			"objections":        []string{"жоқ"},
			"refusal_reason":    nil,
			"mistakes":          []string{"қателік жоқ"},
			"strengths":         []string{"өте жақсы"},
			"next_action":       "хабарласу",
			"criterion_results": map[string]any{
				"custom_intro": map[string]any{"score": 50, "feedback": "Жақсы"},
				"custom_pitch": map[string]any{"score": 50, "feedback": "Керемет"},
			},
		}
		raw, _ := json.Marshal(customJSON)
		writeGeminiText(t, w, string(raw))
	}))
	defer server.Close()

	provider := newTestGemini(t, server.URL, time.Second)
	opts := analysis.Options{
		Criteria: []analysis.CriterionDetail{
			{Key: "custom_intro", Title: "Вводная часть", MaxScore: 50, Description: "Поздороваться"},
			{Key: "custom_pitch", Title: "Презентация", MaxScore: 50, Description: "Рассказать о продукте"},
		},
	}
	result, err := provider.Analyze(context.Background(), transcription.TranscriptResult{Text: "Сәлеметсіз бе"}, opts)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.DetectedLanguage != "kk" {
		t.Errorf("DetectedLanguage = %q, want kk", result.DetectedLanguage)
	}
	if result.CriterionResults["custom_intro"].Score != 50 {
		t.Errorf("custom_intro score = %d, want 50", result.CriterionResults["custom_intro"].Score)
	}
}

func TestGeminiAnalyzeRejectsIncompleteModelAnalysis(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeGeminiText(t, w, `{"summary":"Incomplete analysis"}`)
	}))
	defer server.Close()

	provider := newTestGemini(t, server.URL, time.Second)
	if _, err := provider.Analyze(context.Background(), transcription.TranscriptResult{Text: "transcript text"}); err == nil {
		t.Fatal("Analyze() error = nil, want incomplete model analysis rejection")
	}
}

func TestGeminiUsesConfiguredHTTPTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := newTestGemini(t, server.URL, 10*time.Millisecond)
	_, err := provider.Transcribe(context.Background(), transcription.AudioInput{MIMEType: "audio/wav", Data: []byte("audio")})
	if err == nil {
		t.Fatal("Transcribe() error = nil, want configured client timeout")
	}
	if strings.Contains(err.Error(), "audio") {
		t.Errorf("Transcribe() error = %q, must not expose input content", err)
	}
}

func TestGeminiSanitizesNonSuccessResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "sensitive provider body", http.StatusUnauthorized)
	}))
	defer server.Close()

	provider := newTestGemini(t, server.URL, time.Second)
	_, err := provider.Transcribe(context.Background(), transcription.AudioInput{MIMEType: "audio/wav", Data: []byte("audio")})
	if err == nil {
		t.Fatal("Transcribe() error = nil, want non-2xx error")
	}
	if strings.Contains(err.Error(), "sensitive provider body") || strings.Contains(err.Error(), "test-api-key") {
		t.Errorf("Transcribe() error = %q, must be sanitized", err)
	}
}

func newTestGemini(t *testing.T, baseURL string, timeout time.Duration) *Gemini {
	t.Helper()
	provider, err := NewGemini(GeminiConfig{
		APIKey:             "test-api-key",
		TranscriptionModel: "transcription-model",
		AnalysisModel:      "analysis-model",
		BaseURL:            baseURL,
		Timeout:            timeout,
	})
	if err != nil {
		t.Fatalf("NewGemini() error = %v", err)
	}
	return provider
}

func writeGeminiText(t *testing.T, w http.ResponseWriter, text string) {
	t.Helper()
	response := map[string]any{
		"candidates": []any{map[string]any{
			"content": map[string]any{"parts": []any{map[string]any{"text": text}}},
		}},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		t.Fatalf("encode Gemini response: %v", err)
	}
}

func completeGeminiAnalysisJSON(t *testing.T) string {
	t.Helper()
	payload := map[string]any{
		"summary":        "Summary",
		"needs":          []string{"budget"},
		"objections":     []string{"price"},
		"refusal_reason": nil,
		"mistakes":       []string{"confirm timeline"},
		"strengths":      []string{"rapport"},
		"next_action":    "Send quote",
		"criterion_results": map[string]any{
			"greeting":           map[string]any{"score": 5, "feedback": "good"},
			"rapport":            map[string]any{"score": 10, "feedback": "good"},
			"needs_discovery":    map[string]any{"score": 20, "feedback": "good"},
			"presentation":       map[string]any{"score": 15, "feedback": "good"},
			"objection_handling": map[string]any{"score": 20, "feedback": "good"},
			"next_action":        map[string]any{"score": 15, "feedback": "good"},
			"communication":      map[string]any{"score": 10, "feedback": "good"},
			"closing":            map[string]any{"score": 5, "feedback": "good"},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal complete analysis: %v", err)
	}
	return string(raw)
}
