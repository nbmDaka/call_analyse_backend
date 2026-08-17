package dashboard

import (
	"context"
	"errors"
	"testing"

	"call_analyse_backend/internal/auth"
	"call_analyse_backend/internal/calls"

	"github.com/google/uuid"
)

func TestServicePassesAuthenticatedActorToScopedStore(t *testing.T) {
	managerID := uuid.New()
	store := &fakeStore{summary: func(_ context.Context, actor calls.Actor) (Summary, error) {
		if actor.ID != managerID || actor.Role != auth.RoleManager {
			return Summary{}, errors.New("unscoped actor")
		}
		return Summary{TotalCalls: 1}, nil
	}}
	service := NewService(store)

	summary, err := service.Summary(context.Background(), calls.Actor{ID: managerID, Role: auth.RoleManager})

	if err != nil {
		t.Fatalf("Summary() error = %v", err)
	}
	if summary.TotalCalls != 1 {
		t.Errorf("Summary() = %#v, want scoped store result", summary)
	}
}

func TestServiceRejectsActorWithoutIdentity(t *testing.T) {
	service := NewService(&fakeStore{summary: func(context.Context, calls.Actor) (Summary, error) { return Summary{}, nil }})

	_, err := service.Summary(context.Background(), calls.Actor{Role: auth.RoleManager})

	if !errors.Is(err, calls.ErrInvalidActor) {
		t.Errorf("Summary() error = %v, want calls.ErrInvalidActor", err)
	}
}

type fakeStore struct {
	summary func(context.Context, calls.Actor) (Summary, error)
}

func (s *fakeStore) Summary(ctx context.Context, actor calls.Actor) (Summary, error) {
	return s.summary(ctx, actor)
}
