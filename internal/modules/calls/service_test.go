package calls

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"call_analyse_backend/internal/modules/auth"

	"github.com/google/uuid"
)

func TestServiceCreateUsesBackendGeneratedObjectKey(t *testing.T) {
	store := &memoryCallStore{}
	objects := &memoryObjectStore{}
	service := NewService(store, objects, 100)
	actor := Actor{ID: uuid.New(), Role: auth.RoleManager}

	created, err := service.Create(context.Background(), actor, Upload{
		Filename:    "../../client recording.MP3",
		ContentType: "audio/mpeg",
		Size:        3,
		Reader:      bytes.NewBufferString("mp3"),
	})
	if err == nil {
		t.Fatal("Create() error = nil, want rejected client path")
	}

	created, err = service.Create(context.Background(), actor, Upload{
		Filename:    "client recording.MP3",
		ContentType: "audio/mpeg",
		Size:        3,
		Reader:      bytes.NewBufferString("mp3"),
	})
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if created.Status != StatusUploaded {
		t.Errorf("Create() status = %q, want %q", created.Status, StatusUploaded)
	}
	if !strings.HasPrefix(created.ObjectKey, "calls/"+created.ID.String()+"/") || !strings.HasSuffix(created.ObjectKey, ".mp3") {
		t.Errorf("Create() object key = %q, want server-generated calls/<uuid>/<suffix>.mp3", created.ObjectKey)
	}
	if strings.Contains(created.ObjectKey, "client recording") || strings.ContainsAny(created.ObjectKey, "\\") {
		t.Errorf("Create() object key = %q, must not contain client filename or filesystem separator", created.ObjectKey)
	}
	if got := string(objects.objects[created.ObjectKey]); got != "mp3" {
		t.Errorf("stored data = %q, want %q", got, "mp3")
	}
}

func TestServiceCreateDeletesObjectWhenDatabaseInsertFails(t *testing.T) {
	store := &memoryCallStore{createErr: errors.New("database unavailable")}
	objects := &memoryObjectStore{}
	service := NewService(store, objects, 100)

	_, err := service.Create(context.Background(), Actor{ID: uuid.New(), Role: auth.RoleManager}, Upload{
		Filename:    "recording.wav",
		ContentType: "audio/wav",
		Size:        3,
		Reader:      bytes.NewBufferString("wav"),
	})
	if !errors.Is(err, store.createErr) {
		t.Fatalf("Create() error = %v, want %v", err, store.createErr)
	}
	if len(objects.deleted) != 1 {
		t.Fatalf("Delete() calls = %d, want 1", len(objects.deleted))
	}
	if _, exists := objects.objects[objects.deleted[0]]; exists {
		t.Errorf("object %q still present after database failure", objects.deleted[0])
	}
}

func TestServiceListScopesRowsBeforePagination(t *testing.T) {
	manager := uuid.New()
	supervisor := uuid.New()
	admin := uuid.New()
	store := &memoryCallStore{calls: []Call{
		{ID: uuid.New(), ManagerID: manager, OriginalFilename: "first.mp3"},
		{ID: uuid.New(), ManagerID: uuid.New(), OriginalFilename: "second.mp3"},
	}}
	store.supervisors = map[uuid.UUID]uuid.UUID{manager: supervisor}
	service := NewService(store, &memoryObjectStore{}, 100)

	managerPage, err := service.List(context.Background(), Actor{ID: manager, Role: auth.RoleManager}, Page{Number: 1, Size: 10})
	if err != nil || managerPage.Total != 1 || len(managerPage.Calls) != 1 || managerPage.Calls[0].ManagerID != manager {
		t.Fatalf("manager List() = %#v, %v; want one owned call", managerPage, err)
	}
	supervisorPage, err := service.List(context.Background(), Actor{ID: supervisor, Role: auth.RoleSupervisor}, Page{Number: 1, Size: 10})
	if err != nil || supervisorPage.Total != 1 || len(supervisorPage.Calls) != 1 || supervisorPage.Calls[0].ManagerID != manager {
		t.Fatalf("supervisor List() = %#v, %v; want one supervised-manager call", supervisorPage, err)
	}
	adminPage, err := service.List(context.Background(), Actor{ID: admin, Role: auth.RoleAdmin}, Page{Number: 1, Size: 1})
	if err != nil || adminPage.Total != 2 || len(adminPage.Calls) != 1 || adminPage.TotalPages != 2 {
		t.Fatalf("admin List() = %#v, %v; want total before pagination", adminPage, err)
	}
}

func TestServiceDetailReturnsNotFoundForMissingOrUnownedCall(t *testing.T) {
	owner := uuid.New()
	otherManager := uuid.New()
	supervisor := uuid.New()
	call := Call{ID: uuid.New(), ManagerID: owner}
	service := NewService(&memoryCallStore{calls: []Call{call}, supervisors: map[uuid.UUID]uuid.UUID{owner: supervisor}}, &memoryObjectStore{}, 100)

	for _, actor := range []Actor{
		{ID: otherManager, Role: auth.RoleManager},
		{ID: owner, Role: auth.RoleManager},
		{ID: supervisor, Role: auth.RoleSupervisor},
		{ID: uuid.New(), Role: auth.RoleAdmin},
	} {
		got, err := service.Detail(context.Background(), actor, call.ID)
		if actor.ID == otherManager {
			if !errors.Is(err, ErrCallNotFound) {
				t.Errorf("Detail(unowned call) error = %v, want ErrCallNotFound", err)
			}
			continue
		}
		if err != nil || got.ID != call.ID {
			t.Errorf("Detail(%q) = %#v, %v; want owned/privileged call", actor.Role, got, err)
		}
	}

	if _, err := service.Detail(context.Background(), Actor{ID: owner, Role: auth.RoleManager}, uuid.New()); !errors.Is(err, ErrCallNotFound) {
		t.Errorf("Detail(missing call) error = %v, want ErrCallNotFound", err)
	}
}

func TestListQueryScopesActorBeforePagination(t *testing.T) {
	managerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	query, args, err := listQuery(Actor{ID: managerID, Role: auth.RoleManager}, Page{Number: 2, Size: 25})
	if err != nil {
		t.Fatalf("listQuery() error = %v, want nil", err)
	}
	whereAt := strings.Index(query, "WHERE c.manager_id = $1")
	limitAt := strings.Index(query, "LIMIT $2 OFFSET $3")
	if whereAt < 0 || limitAt < 0 || whereAt > limitAt {
		t.Errorf("listQuery() = %q, want actor scope before pagination", query)
	}
	wantArgs := []any{managerID, 25, 25}
	if len(args) != len(wantArgs) {
		t.Fatalf("listQuery() args = %#v, want %#v", args, wantArgs)
	}
	for i := range wantArgs {
		if args[i] != wantArgs[i] {
			t.Errorf("listQuery() args[%d] = %#v, want %#v", i, args[i], wantArgs[i])
		}
	}
}

type memoryObjectStore struct {
	objects map[string][]byte
	deleted []string
}

func (s *memoryObjectStore) Put(_ context.Context, key string, reader io.Reader, _ int64, _ string) error {
	if s.objects == nil {
		s.objects = make(map[string][]byte)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	s.objects[key] = data
	return nil
}

func (s *memoryObjectStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	data, ok := s.objects[key]
	if !ok {
		return nil, ErrCallNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *memoryObjectStore) Delete(_ context.Context, key string) error {
	delete(s.objects, key)
	s.deleted = append(s.deleted, key)
	return nil
}

type memoryCallStore struct {
	calls       []Call
	supervisors map[uuid.UUID]uuid.UUID
	createErr   error
}

func (s *memoryCallStore) Create(_ context.Context, call Call) (Call, error) {
	if s.createErr != nil {
		return Call{}, s.createErr
	}
	s.calls = append(s.calls, call)
	return call, nil
}

func (s *memoryCallStore) List(_ context.Context, actor Actor, page Page) (CallPage, error) {
	var visible []Call
	for _, call := range s.calls {
		if callVisibleTo(actor, call.ManagerID, s.supervisors) {
			visible = append(visible, call)
		}
	}
	start := (page.Number - 1) * page.Size
	end := start + page.Size
	if start > len(visible) {
		start = len(visible)
	}
	if end > len(visible) {
		end = len(visible)
	}
	totalPages := (len(visible) + page.Size - 1) / page.Size
	return CallPage{Calls: visible[start:end], Total: len(visible), Page: page.Number, PerPage: page.Size, TotalPages: totalPages}, nil
}

func (s *memoryCallStore) Detail(_ context.Context, actor Actor, callID uuid.UUID) (Call, error) {
	for _, call := range s.calls {
		if call.ID == callID && callVisibleTo(actor, call.ManagerID, s.supervisors) {
			return call, nil
		}
	}
	return Call{}, ErrCallNotFound
}

func callVisibleTo(actor Actor, managerID uuid.UUID, supervisors map[uuid.UUID]uuid.UUID) bool {
	switch actor.Role {
	case auth.RoleAdmin:
		return true
	case auth.RoleSupervisor:
		return supervisors[managerID] == actor.ID
	case auth.RoleManager:
		return managerID == actor.ID
	default:
		return false
	}
}
