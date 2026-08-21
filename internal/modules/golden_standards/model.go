// Package golden_standards manages stellar call fragments for training and coaching recommendations.
package golden_standards

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GoldenStandard defines an exemplary sales call moment.
type GoldenStandard struct {
	ID                uuid.UUID  `json:"id"`
	WorkspaceID       uuid.UUID  `json:"workspace_id"`
	CallID            *uuid.UUID `json:"call_id,omitempty"`
	Category          string     `json:"category"`
	Title             string     `json:"title"`
	TranscriptSnippet string     `json:"transcript_snippet"`
	AudioStartSeconds *float64   `json:"audio_start_seconds,omitempty"`
	AudioEndSeconds   *float64   `json:"audio_end_seconds,omitempty"`
	WhyGolden         string     `json:"why_golden"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// Validate ensures required fields are populated.
func (g *GoldenStandard) Validate() error {
	if strings.TrimSpace(g.Category) == "" {
		return fmt.Errorf("category is required")
	}
	if strings.TrimSpace(g.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(g.TranscriptSnippet) == "" {
		return fmt.Errorf("transcript snippet is required")
	}
	if strings.TrimSpace(g.WhyGolden) == "" {
		return fmt.Errorf("why_golden explanation is required")
	}
	return nil
}
