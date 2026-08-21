package calls

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"call_analyse_backend/internal/modules/auth"
	"call_analyse_backend/internal/modules/workspaces"
	"call_analyse_backend/internal/platform/storage"

	"github.com/google/uuid"
)

var (
	// ErrCallNotFound intentionally represents both missing and inaccessible calls.
	ErrCallNotFound = errors.New("call not found")
	ErrInvalidActor = errors.New("invalid call actor")
)

// Actor is the authenticated identity used for call ownership checks.
type Actor struct {
	ID               uuid.UUID // Deprecated legacy identity.
	Role             auth.Role // Deprecated legacy role.
	UserID           uuid.UUID
	WorkspaceID      uuid.UUID
	MembershipID     uuid.UUID
	WorkspaceRole    workspaces.Role
	PlatformRole     workspaces.PlatformRole
	MembershipStatus workspaces.MembershipStatus
	WorkspaceStatus  workspaces.Status
	WorkspaceType    workspaces.Type
}

func ActorFromWorkspace(actor workspaces.Actor) Actor {
	return Actor{UserID: actor.UserID, WorkspaceID: actor.WorkspaceID, MembershipID: actor.MembershipID,
		WorkspaceRole: actor.WorkspaceRole, PlatformRole: actor.PlatformRole,
		MembershipStatus: actor.MembershipStatus, WorkspaceStatus: actor.WorkspaceStatus, WorkspaceType: actor.WorkspaceType}
}

func (a Actor) workspaceActor() workspaces.Actor {
	return workspaces.Actor{UserID: a.UserID, WorkspaceID: a.WorkspaceID, MembershipID: a.MembershipID,
		WorkspaceRole: a.WorkspaceRole, PlatformRole: a.PlatformRole,
		MembershipStatus: a.MembershipStatus, WorkspaceStatus: a.WorkspaceStatus, WorkspaceType: a.WorkspaceType}
}

func (a Actor) userID() uuid.UUID {
	if a.UserID != uuid.Nil {
		return a.UserID
	}
	return a.ID
}

// Upload is a validated-at-the-boundary stream of untrusted client audio.
type Upload struct {
	Filename    string
	ContentType string
	Size        int64
	Reader      io.Reader
}

// Page requests a one-indexed page of calls.
type Page struct {
	Number int
	Size   int
}

// CallPage is a scoped page of calls with total metadata computed before pagination.
type CallPage struct {
	Calls      []Call
	Total      int
	Page       int
	PerPage    int
	TotalPages int
}

// CallStore persists and scopes call metadata.
type CallStore interface {
	Create(ctx context.Context, call Call) (Call, error)
	List(ctx context.Context, actor Actor, page Page) (CallPage, error)
	Detail(ctx context.Context, actor Actor, callID uuid.UUID) (Call, error)
}

// Service coordinates validation, object storage, cleanup, and call metadata.
type Service struct {
	calls    CallStore
	objects  storage.ObjectStore
	maxBytes int64
}

// NewService constructs call application services from focused persistence boundaries.
func NewService(calls CallStore, objects storage.ObjectStore, maxBytes int64) Service {
	return Service{calls: calls, objects: objects, maxBytes: maxBytes}
}

// Create streams a validated upload to object storage, then inserts its metadata.
// If metadata persistence fails, it attempts to remove the just-uploaded object.
func (s Service) Create(ctx context.Context, actor Actor, upload Upload) (Call, error) {
	userID := actor.userID()
	if userID == uuid.Nil {
		return Call{}, ErrInvalidActor
	}
	if actor.WorkspaceID != uuid.Nil && !actor.workspaceActor().CanUpload() {
		if actor.WorkspaceStatus == workspaces.StatusSuspended {
			return Call{}, workspaces.ErrWorkspaceSuspended
		}
		return Call{}, ErrInvalidActor
	}
	if upload.Reader == nil {
		return Call{}, fmt.Errorf("upload reader is required")
	}
	if err := ValidateUpload(upload.Filename, upload.ContentType, upload.Size, s.maxBytes); err != nil {
		return Call{}, err
	}

	call := Call{
		ID:               uuid.New(),
		WorkspaceID:      actor.WorkspaceID,
		OwnerUserID:      userID,
		UploadedByUserID: userID,
		ManagerID:        userID,
		Status:           StatusUploaded,
		OriginalFilename: upload.Filename,
		ContentType:      upload.ContentType,
		SizeBytes:        upload.Size,
	}
	prefix := "calls/"
	if call.WorkspaceID != uuid.Nil {
		prefix = "workspaces/" + call.WorkspaceID.String() + "/calls/"
	}
	call.ObjectKey = prefix + call.ID.String() + "/" + uuid.NewString() + strings.ToLower(filepath.Ext(upload.Filename))

	if err := s.objects.Put(ctx, call.ObjectKey, upload.Reader, call.SizeBytes, call.ContentType); err != nil {
		return Call{}, fmt.Errorf("store call audio: %w", err)
	}
	created, err := s.calls.Create(ctx, call)
	if err != nil {
		if cleanupErr := s.objects.Delete(ctx, call.ObjectKey); cleanupErr != nil {
			return Call{}, errors.Join(fmt.Errorf("create call metadata: %w", err), fmt.Errorf("clean up uploaded call audio: %w", cleanupErr))
		}
		return Call{}, fmt.Errorf("create call metadata: %w", err)
	}
	return created, nil
}

// Queue durably moves a newly created call into the queue checkpoint. The
// optional persistence capability keeps the public CallStore focused while
// allowing the HTTP upload flow to make enqueue state durable.
func (s Service) Queue(ctx context.Context, callID uuid.UUID) error {
	store, ok := s.calls.(interface {
		Get(context.Context, uuid.UUID) (Call, error)
		Transition(context.Context, uuid.UUID, Status, Status, *string) error
	})
	if !ok {
		return fmt.Errorf("call store does not support queue transitions")
	}
	call, err := store.Get(ctx, callID)
	if err != nil {
		return err
	}
	if call.Status == StatusQueued {
		return nil
	}
	return store.Transition(ctx, callID, call.Status, StatusQueued, nil)
}

func (s Service) QueueInWorkspace(ctx context.Context, workspaceID, callID uuid.UUID) error {
	store, ok := s.calls.(interface {
		GetInWorkspace(context.Context, uuid.UUID, uuid.UUID) (Call, error)
		TransitionInWorkspace(context.Context, uuid.UUID, uuid.UUID, Status, Status, *string) error
	})
	if !ok {
		return fmt.Errorf("call store does not support workspace queue transitions")
	}
	call, err := store.GetInWorkspace(ctx, workspaceID, callID)
	if err != nil {
		return err
	}
	if call.Status == StatusQueued {
		return nil
	}
	return store.TransitionInWorkspace(ctx, workspaceID, callID, call.Status, StatusQueued, nil)
}

// FullDetail exposes the enriched read model through the application service.
// Stores without enrichment support still receive a safe processing-pending
// envelope, which keeps test doubles and alternate stores compatible.
func (s Service) FullDetail(ctx context.Context, actor Actor, callID uuid.UUID) (CallDetail, error) {
	if detailed, ok := s.calls.(interface {
		FullDetail(context.Context, Actor, uuid.UUID) (CallDetail, error)
	}); ok {
		return detailed.FullDetail(ctx, actor, callID)
	}
	call, err := s.Detail(ctx, actor, callID)
	if err != nil {
		return CallDetail{}, err
	}
	return CallDetail{Call: call, Audio: AudioMetadata{Filename: call.OriginalFilename, ContentType: call.ContentType, SizeBytes: call.SizeBytes}}, nil
}

// RollbackCreate removes metadata and the object when queue submission fails.
// It is best-effort at the HTTP boundary; the original enqueue error remains
// the response while the durable row cannot be stranded for this failure mode.
func (s Service) RollbackCreate(ctx context.Context, call Call) error {
	if call.WorkspaceID != uuid.Nil {
		if deleter, ok := s.calls.(interface {
			DeleteInWorkspace(context.Context, uuid.UUID, uuid.UUID) error
		}); ok {
			if err := deleter.DeleteInWorkspace(ctx, call.WorkspaceID, call.ID); err != nil {
				return err
			}
		}
	} else if deleter, ok := s.calls.(interface {
		Delete(context.Context, uuid.UUID) error
	}); ok {
		if err := deleter.Delete(ctx, call.ID); err != nil {
			return err
		}
	}
	return s.objects.Delete(ctx, call.ObjectKey)
}

// List delegates scoped selection to the persistence layer before pagination.
func (s Service) List(ctx context.Context, actor Actor, page Page) (CallPage, error) {
	if actor.userID() == uuid.Nil {
		return CallPage{}, ErrInvalidActor
	}
	page, err := normalizePage(page)
	if err != nil {
		return CallPage{}, err
	}
	return s.calls.List(ctx, actor, page)
}

// Detail returns one call only when the scoped store can select it.
func (s Service) Detail(ctx context.Context, actor Actor, callID uuid.UUID) (Call, error) {
	if actor.userID() == uuid.Nil {
		return Call{}, ErrInvalidActor
	}
	call, err := s.calls.Detail(ctx, actor, callID)
	if errors.Is(err, ErrCallNotFound) {
		return Call{}, ErrCallNotFound
	}
	return call, err
}

func normalizePage(page Page) (Page, error) {
	if page.Number == 0 {
		page.Number = 1
	}
	if page.Size == 0 {
		page.Size = 20
	}
	if page.Number < 1 || page.Size < 1 || page.Size > 100 {
		return Page{}, fmt.Errorf("page number must be positive and page size must be between 1 and 100")
	}
	return page, nil
}
