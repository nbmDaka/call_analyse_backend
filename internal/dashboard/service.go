package dashboard

import (
	"context"

	"call_analyse_backend/internal/calls"
)

// Service validates the authenticated actor before requesting scoped aggregates.
type Service struct {
	store Store
}

// NewService constructs dashboard application services from a scoped store.
func NewService(store Store) Service {
	return Service{store: store}
}

// Summary returns the dashboard aggregate visible to actor.
func (s Service) Summary(ctx context.Context, actor calls.Actor) (Summary, error) {
	if actor.ID.String() == "00000000-0000-0000-0000-000000000000" || s.store == nil {
		return Summary{}, calls.ErrInvalidActor
	}
	return s.store.Summary(ctx, actor)
}
