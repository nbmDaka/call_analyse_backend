package providers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"call_analyse_backend/internal/modules/analysis"
	"call_analyse_backend/internal/modules/scoring"
	"call_analyse_backend/internal/modules/transcription"
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
	Logger             *slog.Logger
}

// Gemini implements the transcription and analysis provider boundaries over Gemini HTTP.
type Gemini struct {
	apiKey             string
	transcriptionModel string
	analysisModel      string
	baseURL            string
	httpClient         *http.Client
	logger             *slog.Logger
}

func (g *Gemini) log() *slog.Logger {
	if g.logger != nil {
		return g.logger
	}
	return slog.Default()
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
		logger:             cfg.Logger,
	}, nil
}

// Transcribe submits inline audio to Gemini and extracts the generated transcript text.
func (g *Gemini) Transcribe(ctx context.Context, input transcription.AudioInput) (transcription.TranscriptResult, error) {
	g.log().Info("sending transcription request to Gemini", "model", g.transcriptionModel, "audio_bytes", len(input.Data), "mime_type", input.MIMEType)
	request := geminiGenerateContentRequest{Contents: []geminiContent{{
		Role: "user",
		Parts: []geminiPart{
			{Text: "Выполни транскрибацию аудиозаписи этого звонка продаж. Верни только текст транскрипта без лишних комментариев. Не придумывай таймкоды или имена спикеров, если они недоступны."},
			{InlineData: &geminiInlineData{MIMEType: input.MIMEType, Data: base64.StdEncoding.EncodeToString(input.Data)}},
		},
	}}}
	text, err := g.generate(ctx, g.transcriptionModel, request)
	if err != nil {
		g.log().Error("Gemini transcription failed", "model", g.transcriptionModel, "error", err)
		return transcription.TranscriptResult{}, err
	}
	if strings.TrimSpace(text) == "" {
		g.log().Error("Gemini returned empty transcript text", "model", g.transcriptionModel)
		return transcription.TranscriptResult{}, ErrGeminiRequest
	}
	g.log().Info("Gemini transcription completed", "model", g.transcriptionModel, "transcript_chars", len(text))
	return transcription.TranscriptResult{Text: text}, nil
}

// Analyze requests structured JSON and decodes the model's returned analysis.
// Strict business validation is deliberately owned by the analysis package.
func (g *Gemini) Analyze(ctx context.Context, transcript transcription.Transcript, opts ...analysis.Options) (analysis.Analysis, error) {
	g.log().Info("sending analysis request to Gemini", "model", g.analysisModel, "transcript_chars", len(transcript.Text))

	var criteriaList []analysis.CriterionDetail
	if len(opts) > 0 && len(opts[0].Criteria) > 0 {
		criteriaList = opts[0].Criteria
	} else {
		criteriaList = []analysis.CriterionDetail{
			{Key: "greeting", Title: "Приветствие", MaxScore: 5, Description: "Менеджер должен четко поздороваться, назвать свое имя и компанию."},
			{Key: "rapport", Title: "Установление контакта", MaxScore: 10, Description: "Вежливый и доброжелательный тон, обращение к клиенту по имени."},
			{Key: "needs_discovery", Title: "Выявление потребностей", MaxScore: 20, Description: "Задать минимум 2-3 открытых вопроса для понимания задачи и бюджета клиента."},
			{Key: "presentation", Title: "Презентация", MaxScore: 15, Description: "Презентовать продукт через выгоды для клиента, а не просто перечислять функции."},
			{Key: "objection_handling", Title: "Работа с возражениями", MaxScore: 20, Description: "Выслушать сомнения клиента, согласиться с важностью вопроса и аргументированно отработать."},
			{Key: "next_action", Title: "Следующий шаг", MaxScore: 15, Description: "Назначить конкретную дату, время и цель следующего контакта."},
			{Key: "communication", Title: "Коммуникация", MaxScore: 10, Description: "Отсутствие слов-паразитов, перебиваний и пассивной агрессии."},
			{Key: "closing", Title: "Завершение сделки", MaxScore: 5, Description: "Позитивное прощание и подтверждение договоренностей."},
		}
	}

	var criteriaLines strings.Builder
	var scoringCriteria []scoring.Criterion
	var criterionKeys []string
	for _, c := range criteriaList {
		scoringCriteria = append(scoringCriteria, scoring.Criterion{Key: c.Key, Max: c.MaxScore})
		criterionKeys = append(criterionKeys, fmt.Sprintf("%q", c.Key))
		desc := c.Description
		if strings.TrimSpace(desc) == "" {
			desc = c.Title
		}
		criteriaLines.WriteString(fmt.Sprintf("- %s (%s, макс. %d баллов): %s\n", c.Key, c.Title, c.MaxScore, desc))
	}

	prompt := `Проанализируй следующий транскрипт звонка менеджера по продажам.
Оцени разговор по критериям чек-листа:
` + criteriaLines.String() + `
Также определи динамику речи (баланс речи менеджера и клиента в процентах talk-to-listen, неловкие паузы >3.5 сек, перебивания, эмоциональный тон), распределение ролей (кто менеджер, а кто клиент), выявленные ошибки и нарушения с уровнем критичности, а также персональные практические рекомендации для менеджера.

ЯЗЫКОВЫЕ ПРАВИЛА (LANGUAGE INSTRUCTIONS):
1. Определи язык звонка: "ru" (русский), "kk" (казахский) или "mixed" (смешанный / код-свитчинг). Запиши его в поле "detected_language".
2. Язык всего анализа (summary, feedback по критериям, needs, objections, mistakes, strengths, next_action, violations.title, violations.fix_advice, actionable_coaching, emotional_tone) ДОЛЖЕН СТРОГО СООТВЕТСТВОВАТЬ языку звонка:
   - Если звонок на казахском языке -> ВСЕ текстовые поля пиши на казахском языке (қазақ тілінде).
   - Если звонок на русском языке -> ВСЕ текстовые поля пиши на русском языке.
   - Если звонок смешанный (mixed) -> пиши на русском языке (или на основном языке клиента).

Верни ответ ТОЛЬКО в формате JSON со следующими полями:
- summary (string, резюме звонка на языке разговора)
- detected_language (string, "ru" | "kk" | "mixed")
- needs (array of strings, выявленные потребности клиента)
- objections (array of strings, возражения клиента)
- refusal_reason (nullable string, причина отказа клиента, если была, иначе null)
- mistakes (array of strings, ошибки менеджера)
- strengths (array of strings, сильные стороны менеджера)
- next_action (string, рекомендуемое следующее действие)
- criterion_results (объект, содержащий ключи ` + strings.Join(criterionKeys, ", ") + `, где для каждого ключа указан объект {score: number, feedback: string с подробным обоснованием оценки})
- role_mapping ({manager_speaker: string, client_speaker: string})
- speech_analytics ({talk_to_listen: {manager_percentage: number, client_percentage: number}, awkward_pauses: [{start_seconds: number, end_seconds: number, duration_seconds: number}], interruptions: [{timestamp_seconds: number, interrupted_by: string, context: string}], emotional_tone: {manager_tone: string, client_tone: string, sentiment_shift: string}})
- violations (массив объектов {severity: "low"|"medium"|"high", title: string (название ошибки), quote: string (цитата из транскрипта), timestamp_seconds: number, fix_advice: string (как исправить ошибку)})
- actionable_coaching (массив строк с практическими тактическими рекомендациями для менеджера)

Транскрипт звонка:
` + transcript.Text
	request := geminiGenerateContentRequest{
		Contents:         []geminiContent{{Role: "user", Parts: []geminiPart{{Text: prompt}}}},
		GenerationConfig: &geminiGenerationConfig{ResponseMIMEType: "application/json"},
	}
	text, err := g.generate(ctx, g.analysisModel, request)
	if err != nil {
		g.log().Error("Gemini analysis failed", "model", g.analysisModel, "error", err)
		return analysis.Analysis{}, err
	}
	result, err := analysis.ParseAndValidateWithCriteria([]byte(text), scoringCriteria)
	if err != nil {
		g.log().Error("Gemini analysis parsing/validation failed", "model", g.analysisModel, "error", err, "raw_response", text)
		return analysis.Analysis{}, ErrGeminiRequest
	}
	g.log().Info("Gemini analysis successfully decoded and validated", "model", g.analysisModel, "detected_language", result.DetectedLanguage)
	return result, nil
}


func (g *Gemini) generate(ctx context.Context, model string, request geminiGenerateContentRequest) (string, error) {
	body, err := json.Marshal(request)
	if err != nil {
		g.log().Error("failed to marshal Gemini request payload", "error", err)
		return "", ErrGeminiRequest
	}

	maxAttempts := 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint(model), bytes.NewReader(body))
		if err != nil {
			g.log().Error("failed to create HTTP request for Gemini", "error", err)
			return "", ErrGeminiRequest
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-goog-api-key", g.apiKey)

		response, err := g.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			g.log().Error("Gemini HTTP call failed", "model", model, "attempt", attempt, "error", err)
			if attempt < maxAttempts {
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(time.Duration(attempt) * time.Second):
					continue
				}
			}
			return "", ErrGeminiRequest
		}
		responseBody, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil {
			g.log().Error("failed to read Gemini response body", "model", model, "error", readErr)
			return "", ErrGeminiRequest
		}

		if response.StatusCode == http.StatusServiceUnavailable || response.StatusCode == http.StatusTooManyRequests || response.StatusCode == http.StatusGatewayTimeout {
			g.log().Warn("Gemini temporary error, retrying", "model", model, "attempt", attempt, "status", response.StatusCode, "body", string(responseBody))
			if attempt < maxAttempts {
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(time.Duration(attempt) * time.Second):
					continue
				}
			}
			return "", fmt.Errorf("%w: status %d", ErrGeminiRequest, response.StatusCode)
		}

		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			g.log().Error("Gemini returned non-2xx status code", "model", model, "status", response.StatusCode, "body", string(responseBody))
			return "", fmt.Errorf("%w: status %d", ErrGeminiRequest, response.StatusCode)
		}

		var decoded geminiGenerateContentResponse
		if err := json.Unmarshal(responseBody, &decoded); err != nil {
			g.log().Error("failed to unmarshal Gemini JSON response", "model", model, "error", err, "body", string(responseBody))
			return "", ErrGeminiRequest
		}
		text := decoded.text()
		if text == "" {
			g.log().Error("Gemini response contained no candidate parts", "model", model, "body", string(responseBody))
			return "", ErrGeminiRequest
		}
		return text, nil
	}
	return "", ErrGeminiRequest
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
