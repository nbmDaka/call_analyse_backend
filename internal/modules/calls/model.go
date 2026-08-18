// Package calls contains call domain types and status-transition rules.
package calls

import (
	"call_analyse_backend/internal/modules/analysis"
	"call_analyse_backend/internal/modules/auth"
	"call_analyse_backend/internal/modules/scoring"
	"call_analyse_backend/internal/modules/transcription"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Status is the lifecycle state of an uploaded call.
type Status string

const (
	StatusUploaded     Status = "uploaded"
	StatusQueued       Status = "queued"
	StatusTranscribing Status = "transcribing"
	StatusTranscribed  Status = "transcribed"
	StatusAnalyzing    Status = "analyzing"
	StatusCompleted    Status = "completed"
	StatusFailed       Status = "failed"
)

// Call is the persisted metadata for an uploaded sales call.
type Call struct {
	ID               uuid.UUID
	ManagerID        uuid.UUID
	Status           Status
	OriginalFilename string
	ObjectKey        string
	ContentType      string
	SizeBytes        int64
	DurationSeconds  *int
	ErrorMessage     *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// CallDetail is the API read model. Result fields are nullable because a call
// may be returned while asynchronous processing is still in progress.
type CallDetail struct {
	Call       Call                      `json:"call"`
	Manager    *auth.PublicUser          `json:"manager,omitempty"`
	Audio      AudioMetadata             `json:"audio"`
	Transcript *transcription.Transcript `json:"transcript,omitempty"`
	Analysis   *analysis.Analysis        `json:"analysis,omitempty"`
	Score      *scoring.Score            `json:"score,omitempty"`
}

type AudioMetadata struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

var allowedTransitions = map[Status]map[Status]struct{}{
	StatusUploaded: {
		StatusQueued:       {},
		StatusTranscribing: {},
	},
	StatusQueued: {
		StatusTranscribing: {},
		StatusFailed:       {},
	},
	StatusTranscribing: {
		StatusTranscribed: {},
		StatusFailed:      {},
	},
	StatusTranscribed: {
		StatusAnalyzing: {},
		StatusFailed:    {},
	},
	StatusAnalyzing: {
		StatusCompleted: {},
		StatusFailed:    {},
	},
	StatusFailed: {
		StatusQueued: {},
	},
}

// CanTransition reports whether a call may move from one status to another.
func CanTransition(from, to Status) bool {
	_, ok := allowedTransitions[from][to]
	return ok
}

// ValidateTransition returns an error when a call status change is invalid.
func ValidateTransition(from, to Status) error {
	if !CanTransition(from, to) {
		return fmt.Errorf("invalid call status transition: %s -> %s", from, to)
	}
	return nil
}
