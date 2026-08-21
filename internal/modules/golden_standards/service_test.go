package golden_standards

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type memoryStore struct {
	items map[uuid.UUID]GoldenStandard
}

func newMemoryStore() *memoryStore {
	return &memoryStore{items: make(map[uuid.UUID]GoldenStandard)}
}

func (m *memoryStore) List(_ context.Context, workspaceID uuid.UUID, category string) ([]GoldenStandard, error) {
	var list []GoldenStandard
	for _, g := range m.items {
		if g.WorkspaceID == workspaceID {
			if category == "" || g.Category == category {
				list = append(list, g)
			}
		}
	}
	return list, nil
}

func (m *memoryStore) GetByID(_ context.Context, workspaceID, id uuid.UUID) (GoldenStandard, error) {
	g, ok := m.items[id]
	if !ok || g.WorkspaceID != workspaceID {
		return GoldenStandard{}, ErrGoldenStandardNotFound
	}
	return g, nil
}

func (m *memoryStore) Create(_ context.Context, g GoldenStandard) (GoldenStandard, error) {
	g.ID = uuid.New()
	g.CreatedAt = time.Now()
	g.UpdatedAt = time.Now()
	m.items[g.ID] = g
	return g, nil
}

func (m *memoryStore) Delete(_ context.Context, workspaceID, id uuid.UUID) error {
	g, ok := m.items[id]
	if !ok || g.WorkspaceID != workspaceID {
		return ErrGoldenStandardNotFound
	}
	delete(m.items, id)
	return nil
}

func TestGoldenStandardsService(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	svc := NewService(store)
	wsID := uuid.New()

	// 1. Create
	startSec := 15.0
	endSec := 35.5
	created, err := svc.Create(ctx, wsID, CreateInput{
		Category:          "objection_handling",
		Title:             "Great response to price objection",
		TranscriptSnippet: "I understand price is a factor, let's explore ROI...",
		AudioStartSeconds: &startSec,
		AudioEndSeconds:   &endSec,
		WhyGolden:         "Polite, value-oriented negotiation.",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Title != "Great response to price objection" {
		t.Fatalf("Create() unexpected title = %q", created.Title)
	}

	// 2. GetByID
	got, err := svc.GetByID(ctx, wsID, created.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("GetByID() got ID %v, want %v", got.ID, created.ID)
	}

	// 3. List
	list, err := svc.List(ctx, wsID, "objection_handling")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List() length = %d, want 1", len(list))
	}

	// 4. Delete
	if err := svc.Delete(ctx, wsID, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := svc.GetByID(ctx, wsID, created.ID); err != ErrGoldenStandardNotFound {
		t.Fatalf("GetByID() error = %v, want ErrGoldenStandardNotFound", err)
	}
}
