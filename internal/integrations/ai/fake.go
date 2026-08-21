// Package providers contains adapters for external AI services and development fakes.
package providers

import (
	"context"

	"call_analyse_backend/internal/modules/analysis"
	"call_analyse_backend/internal/modules/scoring"
	"call_analyse_backend/internal/modules/transcription"
)

// FakeAnalysisProvider provides deterministic analysis for development and tests.
type FakeAnalysisProvider struct{}

// Analyze returns fixed, complete structured analysis with every scoring criterion.
func (FakeAnalysisProvider) Analyze(_ context.Context, _ transcription.Transcript) (analysis.Analysis, error) {
	criteria := make(map[string]analysis.CriterionResult, len(scoring.Criteria()))
	for _, criterion := range scoring.Criteria() {
		criteria[criterion.Key] = analysis.CriterionResult{Score: criterion.Max / 2, Feedback: "Deterministic fake feedback."}
	}
	ts := 45.5
	return analysis.Analysis{
		Summary:          "Клиент запросил индивидуальное коммерческое предложение с учетом бюджета.",
		Needs:            []string{"Коммерческое предложение в рамках бюджета", "Сроки внедрения до 1 месяца"},
		Objections:       []string{"Чувствительность к цене"},
		RefusalReason:    nil,
		Mistakes:         []string{"Уточнить график принятия решений."},
		Strengths:        []string{"Четко зафиксированы потребности клиента."},
		NextAction:       "Отправить адаптированное КП во вторник до 15:00.",
		CriterionResults: criteria,
		RoleMapping: &analysis.RoleMapping{
			ManagerSpeaker: "Speaker 0",
			ClientSpeaker:  "Speaker 1",
		},
		SpeechAnalytics: &analysis.SpeechAnalytics{
			TalkToListen: &analysis.TalkToListenRatio{
				ManagerPercentage: 60,
				ClientPercentage:  40,
			},
			AwkwardPauses: []analysis.AwkwardPause{
				{StartSeconds: 15.2, EndSeconds: 19.0, DurationSeconds: 3.8},
			},
			Interruptions: []analysis.Interruption{
				{TimestampSeconds: 42.0, InterruptedBy: "manager", Context: "Менеджер начал озвучивать условия до окончания вопроса клиента"},
			},
			EmotionalTone: &analysis.EmotionalTone{
				ManagerTone:    "доброжелательный, уверенный",
				ClientTone:     "заинтересованный",
				SentimentShift: "positive",
			},
		},
		Violations: []analysis.Violation{
			{
				Severity:         "medium",
				Title:            "Преждевременный переход к ценообразованию",
				Quote:            "Наш тариф стоит 50 000 в месяц...",
				TimestampSeconds: &ts,
				FixAdvice:        "Сначала завершите квалификацию потребностей перед озвучиванием цены.",
			},
		},
		ActionableCoaching: []string{
			"Делайте паузу 2-3 секунды после реплики клиента, чтобы избежать перебиваний.",
			"Задайте минимум 2 вопроса о бизнес-процессах клиента до презентации цен.",
		},
	}, nil
}

var _ analysis.AnalysisProvider = FakeAnalysisProvider{}
