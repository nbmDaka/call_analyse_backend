package playbooks

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type memoryStore struct {
	items map[uuid.UUID]Playbook
}

func newMemoryStore() *memoryStore {
	return &memoryStore{items: make(map[uuid.UUID]Playbook)}
}

func (m *memoryStore) List(_ context.Context, workspaceID uuid.UUID) ([]Playbook, error) {
	var list []Playbook
	for _, p := range m.items {
		if p.WorkspaceID == workspaceID {
			list = append(list, p)
		}
	}
	return list, nil
}

func (m *memoryStore) GetByID(_ context.Context, workspaceID, id uuid.UUID) (Playbook, error) {
	p, ok := m.items[id]
	if !ok || p.WorkspaceID != workspaceID {
		return Playbook{}, ErrPlaybookNotFound
	}
	return p, nil
}

func (m *memoryStore) GetDefault(_ context.Context, workspaceID uuid.UUID) (Playbook, error) {
	for _, p := range m.items {
		if p.WorkspaceID == workspaceID && p.IsDefault {
			return p, nil
		}
	}
	for _, p := range m.items {
		if p.WorkspaceID == workspaceID {
			return p, nil
		}
	}
	return Playbook{}, ErrPlaybookNotFound
}

func (m *memoryStore) Create(_ context.Context, p Playbook) (Playbook, error) {
	p.ID = uuid.New()
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	if p.IsDefault {
		for k, item := range m.items {
			if item.WorkspaceID == p.WorkspaceID {
				item.IsDefault = false
				m.items[k] = item
			}
		}
	}
	m.items[p.ID] = p
	return p, nil
}

func (m *memoryStore) Update(_ context.Context, p Playbook) (Playbook, error) {
	p.UpdatedAt = time.Now()
	if p.IsDefault {
		for k, item := range m.items {
			if item.WorkspaceID == p.WorkspaceID && item.ID != p.ID {
				item.IsDefault = false
				m.items[k] = item
			}
		}
	}
	m.items[p.ID] = p
	return p, nil
}

func (m *memoryStore) Delete(_ context.Context, workspaceID, id uuid.UUID) error {
	p, ok := m.items[id]
	if !ok || p.WorkspaceID != workspaceID {
		return ErrPlaybookNotFound
	}
	delete(m.items, id)
	return nil
}

func TestPlaybookService(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	svc := NewService(store)
	wsID := uuid.New()

	// 1. Create with defaults
	created, err := svc.Create(ctx, wsID, CreateInput{
		Name:        "Test Playbook",
		Description: "Standard rules",
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Name != "Test Playbook" || len(created.Criteria) != 8 || !created.IsDefault {
		t.Fatalf("Create() returned unexpected playbook: %+v", created)
	}

	// 2. GetDefault
	def, err := svc.GetDefault(ctx, wsID)
	if err != nil {
		t.Fatalf("GetDefault() error = %v", err)
	}
	if def.ID != created.ID {
		t.Fatalf("GetDefault() got %v, want %v", def.ID, created.ID)
	}

	// 3. Update
	newName := "Updated Playbook"
	updated, err := svc.Update(ctx, wsID, created.ID, UpdateInput{
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != newName {
		t.Fatalf("Update() name = %q, want %q", updated.Name, newName)
	}

	// 4. List
	list, err := svc.List(ctx, wsID)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List() length = %d, want 1", len(list))
	}

	// 5. Delete
	if err := svc.Delete(ctx, wsID, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := svc.GetByID(ctx, wsID, created.ID); err != ErrPlaybookNotFound {
		t.Fatalf("GetByID() error = %v, want ErrPlaybookNotFound", err)
	}
}
