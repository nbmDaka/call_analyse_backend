package playbooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// CreateInput defines parameters for creating a new Playbook.
type CreateInput struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	IsDefault   bool        `json:"is_default"`
	Criteria    []Criterion `json:"criteria"`
}

// UpdateInput defines parameters for updating an existing Playbook.
type UpdateInput struct {
	Name        *string      `json:"name"`
	Description *string      `json:"description"`
	IsDefault   *bool        `json:"is_default"`
	Criteria    *[]Criterion `json:"criteria"`
}

// Service owns business logic and validation for workspace playbooks.
type Service struct {
	store Store
}

// NewService constructs a Playbook Service.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// List returns all playbooks for a workspace.
func (s *Service) List(ctx context.Context, workspaceID uuid.UUID) ([]Playbook, error) {
	if workspaceID == uuid.Nil {
		return nil, fmt.Errorf("workspace ID is required")
	}
	return s.store.List(ctx, workspaceID)
}

// GetByID returns one playbook by ID within a workspace.
func (s *Service) GetByID(ctx context.Context, workspaceID, id uuid.UUID) (Playbook, error) {
	if workspaceID == uuid.Nil {
		return Playbook{}, fmt.Errorf("workspace ID is required")
	}
	if id == uuid.Nil {
		return Playbook{}, fmt.Errorf("playbook ID is required")
	}
	return s.store.GetByID(ctx, workspaceID, id)
}

// GetDefault returns the default active playbook for a workspace.
func (s *Service) GetDefault(ctx context.Context, workspaceID uuid.UUID) (Playbook, error) {
	if workspaceID == uuid.Nil {
		return Playbook{}, fmt.Errorf("workspace ID is required")
	}
	return s.store.GetDefault(ctx, workspaceID)
}

// Create validates and saves a new playbook for a workspace.
func (s *Service) Create(ctx context.Context, workspaceID uuid.UUID, input CreateInput) (Playbook, error) {
	if workspaceID == uuid.Nil {
		return Playbook{}, fmt.Errorf("workspace ID is required")
	}
	criteria := input.Criteria
	if len(criteria) == 0 {
		criteria = DefaultCriteria()
	}
	pb := Playbook{
		WorkspaceID: workspaceID,
		Name:        strings.TrimSpace(input.Name),
		Description: strings.TrimSpace(input.Description),
		IsDefault:   input.IsDefault,
		Criteria:    criteria,
	}
	if err := pb.Validate(); err != nil {
		return Playbook{}, err
	}
	return s.store.Create(ctx, pb)
}

// Update validates and updates an existing playbook.
func (s *Service) Update(ctx context.Context, workspaceID, id uuid.UUID, input UpdateInput) (Playbook, error) {
	if workspaceID == uuid.Nil {
		return Playbook{}, fmt.Errorf("workspace ID is required")
	}
	if id == uuid.Nil {
		return Playbook{}, fmt.Errorf("playbook ID is required")
	}
	existing, err := s.store.GetByID(ctx, workspaceID, id)
	if err != nil {
		return Playbook{}, err
	}

	if input.Name != nil {
		existing.Name = strings.TrimSpace(*input.Name)
	}
	if input.Description != nil {
		existing.Description = strings.TrimSpace(*input.Description)
	}
	if input.IsDefault != nil {
		existing.IsDefault = *input.IsDefault
	}
	if input.Criteria != nil {
		existing.Criteria = *input.Criteria
	}

	if err := existing.Validate(); err != nil {
		return Playbook{}, err
	}
	return s.store.Update(ctx, existing)
}

// Delete removes a playbook from a workspace.
func (s *Service) Delete(ctx context.Context, workspaceID, id uuid.UUID) error {
	if workspaceID == uuid.Nil {
		return fmt.Errorf("workspace ID is required")
	}
	if id == uuid.Nil {
		return fmt.Errorf("playbook ID is required")
	}
	return s.store.Delete(ctx, workspaceID, id)
}
