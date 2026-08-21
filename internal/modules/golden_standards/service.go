package golden_standards

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// CreateInput defines parameters for adding a new Golden Standard.
type CreateInput struct {
	CallID            *uuid.UUID `json:"call_id,omitempty"`
	Category          string     `json:"category"`
	Title             string     `json:"title"`
	TranscriptSnippet string     `json:"transcript_snippet"`
	AudioStartSeconds *float64   `json:"audio_start_seconds,omitempty"`
	AudioEndSeconds   *float64   `json:"audio_end_seconds,omitempty"`
	WhyGolden         string     `json:"why_golden"`
}

// Service owns business logic and validation for workspace golden standards.
type Service struct {
	store Store
}

// NewService constructs a Golden Standards Service.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// List returns golden standards for a workspace.
func (s *Service) List(ctx context.Context, workspaceID uuid.UUID, category string) ([]GoldenStandard, error) {
	if workspaceID == uuid.Nil {
		return nil, fmt.Errorf("workspace ID is required")
	}
	return s.store.List(ctx, workspaceID, strings.TrimSpace(category))
}

// GetByID returns one golden standard.
func (s *Service) GetByID(ctx context.Context, workspaceID, id uuid.UUID) (GoldenStandard, error) {
	if workspaceID == uuid.Nil {
		return GoldenStandard{}, fmt.Errorf("workspace ID is required")
	}
	if id == uuid.Nil {
		return GoldenStandard{}, fmt.Errorf("golden standard ID is required")
	}
	return s.store.GetByID(ctx, workspaceID, id)
}

// Create validates and saves a new golden standard.
func (s *Service) Create(ctx context.Context, workspaceID uuid.UUID, input CreateInput) (GoldenStandard, error) {
	if workspaceID == uuid.Nil {
		return GoldenStandard{}, fmt.Errorf("workspace ID is required")
	}
	standard := GoldenStandard{
		WorkspaceID:       workspaceID,
		CallID:            input.CallID,
		Category:          strings.TrimSpace(input.Category),
		Title:             strings.TrimSpace(input.Title),
		TranscriptSnippet: strings.TrimSpace(input.TranscriptSnippet),
		AudioStartSeconds: input.AudioStartSeconds,
		AudioEndSeconds:   input.AudioEndSeconds,
		WhyGolden:         strings.TrimSpace(input.WhyGolden),
	}
	if err := standard.Validate(); err != nil {
		return GoldenStandard{}, err
	}
	return s.store.Create(ctx, standard)
}

// Delete removes a golden standard from a workspace.
func (s *Service) Delete(ctx context.Context, workspaceID, id uuid.UUID) error {
	if workspaceID == uuid.Nil {
		return fmt.Errorf("workspace ID is required")
	}
	if id == uuid.Nil {
		return fmt.Errorf("golden standard ID is required")
	}
	return s.store.Delete(ctx, workspaceID, id)
}
