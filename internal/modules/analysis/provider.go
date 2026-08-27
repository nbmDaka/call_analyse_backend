package analysis

import (
	"context"

	"call_analyse_backend/internal/modules/transcription"
)

// CriterionDetail defines a rubric rule for LLM evaluation.
type CriterionDetail struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Description string `json:"description"`
	MaxScore    int    `json:"max_score"`
}

// Options configures runtime analysis parameters such as dynamic playbook criteria.
type Options struct {
	Criteria []CriterionDetail
	Language string
}

// AnalysisProvider derives structured coaching data from a transcript.
type AnalysisProvider interface {
	Analyze(context.Context, transcription.Transcript, ...Options) (Analysis, error)
}

