package analysis

import (
	"context"

	"call_analyse_backend/internal/modules/transcription"
)

// AnalysisProvider derives structured coaching data from a transcript.
type AnalysisProvider interface {
	Analyze(context.Context, transcription.Transcript) (Analysis, error)
}
