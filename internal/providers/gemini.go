package providers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"call_analyse_backend/internal/analysis"
	"call_analyse_backend/internal/transcription"
)

const defaultGeminiBaseURL = "https://generativelanguage.googleapis.com"

var ErrGeminiRequest = errors.New("Gemini provider request failed")

// GeminiConfig contains only the configuration required by the Gemini adapter.
type GeminiConfig struct {
	APIKey             string
	TranscriptionModel string
	AnalysisModel      string
	BaseURL            string
	Timeout            time.Duration
}

// Gemini implements the transcription and analysis provider boundaries over Gemini HTTP.
type Gemini struct {
	apiKey             string
	transcriptionModel string
	analysisModel      string
	baseURL            string
	httpClient         *http.Client
}

// NewGemini constructs an adapter with an explicit HTTP timeout.
func NewGemini(cfg GeminiConfig) (*Gemini, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("Gemini API key is required")
	}
	if strings.TrimSpace(cfg.TranscriptionModel) == "" {
		return nil, fmt.Errorf("Gemini transcription model is required")
	}
	if strings.TrimSpace(cfg.AnalysisModel) == "" {
		return nil, fmt.Errorf("Gemini analysis model is required")
	}
	if cfg.Timeout <= 0 {
		return nil, fmt.Errorf("Gemini timeout must be greater than zero")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultGeminiBaseURL
	}
	return &Gemini{
		apiKey:             cfg.APIKey,
		transcriptionModel: cfg.TranscriptionModel,
		analysisModel:      cfg.AnalysisModel,
		baseURL:            baseURL,
		httpClient:         &http.Client{Timeout: cfg.Timeout},
	}, nil
}

// Transcribe submits inline audio to Gemini and extracts the generated transcript text.
func (g *Gemini) Transcribe(ctx context.Context, input transcription.AudioInput) (transcription.TranscriptResult, error) {
	request := geminiGenerateContentRequest{Contents: []geminiContent{{
		Role: "user",
		Parts: []geminiPart{
			{Text: "Transcribe this sales-call audio. Return only the transcript text. Do not invent timestamps or speaker labels when unavailable."},
			{InlineData: &geminiInlineData{MIMEType: input.MIMEType, Data: base64.StdEncoding.EncodeToString(input.Data)}},
		},
	}}}
	text, err := g.generate(ctx, g.transcriptionModel, request)
	if err != nil {
		return transcription.TranscriptResult{}, err
	}
	if strings.TrimSpace(text) == "" {
		return transcription.TranscriptResult{}, ErrGeminiRequest
	}
	return transcription.TranscriptResult{Text: text}, nil
}

// Analyze requests structured JSON and decodes the model's returned analysis.
// Strict business validation is deliberately owned by the analysis package.
func (g *Gemini) Analyze(ctx context.Context, transcript transcription.Transcript) (analysis.Analysis, error) {
	prompt := `Analyze the following sales-call transcript. Return JSON only with these fields: summary, needs, objections, refusal_reason, mistakes, strengths, next_action, criterion_results. criterion_results must map each criterion to an object with score and feedback. Transcript:
` + transcript.Text
	request := geminiGenerateContentRequest{
		Contents:         []geminiContent{{Role: "user", Parts: []geminiPart{{Text: prompt}}}},
		GenerationConfig: &geminiGenerationConfig{ResponseMIMEType: "application/json"},
	}
	text, err := g.generate(ctx, g.analysisModel, request)
	if err != nil {
		return analysis.Analysis{}, err
	}
	result, err := analysis.ParseAndValidate([]byte(text))
	if err != nil {
		return analysis.Analysis{}, ErrGeminiRequest
	}
	return result, nil
}

func (g *Gemini) generate(ctx context.Context, model string, request geminiGenerateContentRequest) (string, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return "", ErrGeminiRequest
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint(model), bytes.NewReader(body))
	if err != nil {
		return "", ErrGeminiRequest
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", g.apiKey)

	response, err := g.httpClient.Do(req)
	if err != nil {
		return "", ErrGeminiRequest
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("%w: status %d", ErrGeminiRequest, response.StatusCode)
	}
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", ErrGeminiRequest
	}
	var decoded geminiGenerateContentResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return "", ErrGeminiRequest
	}
	text := decoded.text()
	if text == "" {
		return "", ErrGeminiRequest
	}
	return text, nil
}

func (g *Gemini) endpoint(model string) string {
	return g.baseURL + "/v1beta/models/" + model + ":generateContent"
}

type geminiGenerateContentRequest struct {
	Contents         []geminiContent         `json:"contents"`
	GenerationConfig *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiGenerationConfig struct {
	ResponseMIMEType string `json:"response_mime_type"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inline_data,omitempty"`
}

type geminiInlineData struct {
	MIMEType string `json:"mime_type"`
	Data     string `json:"data"`
}

type geminiGenerateContentResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
}

type geminiCandidate struct {
	Content geminiContent `json:"content"`
}

func (r geminiGenerateContentResponse) text() string {
	if len(r.Candidates) == 0 {
		return ""
	}
	var text strings.Builder
	for _, part := range r.Candidates[0].Content.Parts {
		text.WriteString(part.Text)
	}
	return strings.TrimSpace(text.String())
}

var (
	_ transcription.TranscriptionProvider = (*Gemini)(nil)
	_ analysis.AnalysisProvider           = (*Gemini)(nil)
)
