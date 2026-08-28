package invitations

import (
	"context"
	"errors"
	"testing"
	"time"

	"call_analyse_backend/internal/modules/auth"
	"call_analyse_backend/internal/modules/workspaces"
	"github.com/google/uuid"
)

type fakeStore struct {
	createdInvitation Invitation
	pendingList       []Invitation
	invitationInfo    Invitation
	isExisting        bool
	getErr            error
	acceptedWsID      uuid.UUID
	acceptErr         error
	registeredUser    auth.User
	registerWsID      uuid.UUID
	registerErr       error
}

func (f *fakeStore) Create(ctx context.Context, inv Invitation, tokenHash string) (Invitation, error) {
	f.createdInvitation = inv
	inv.WorkspaceName = "Test Company"
	return inv, nil
}

func (f *fakeStore) ListPendingByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]Invitation, error) {
	return f.pendingList, nil
}

func (f *fakeStore) GetByTokenHash(ctx context.Context, tokenHash string) (Invitation, bool, error) {
	if f.getErr != nil {
		return Invitation{}, false, f.getErr
	}
	return f.invitationInfo, f.isExisting, nil
}

func (f *fakeStore) Revoke(ctx context.Context, workspaceID, invitationID uuid.UUID) error {
	return nil
}

func (f *fakeStore) AcceptForUser(ctx context.Context, tokenHash string, userID uuid.UUID) (uuid.UUID, error) {
	if f.acceptErr != nil {
		return uuid.Nil, f.acceptErr
	}
	return f.acceptedWsID, nil
}

func (f *fakeStore) RegisterAndAccept(ctx context.Context, tokenHash string, passwordHash string) (auth.User, uuid.UUID, error) {
	if f.registerErr != nil {
		return auth.User{}, uuid.Nil, f.registerErr
	}
	return f.registeredUser, f.registerWsID, nil
}

type fakeEmailer struct {
	sentTo   string
	subject  string
	htmlBody string
}

func (f *fakeEmailer) Send(ctx context.Context, to, subject, htmlBody, textBody string) error {
	f.sentTo = to
	f.subject = subject
	f.htmlBody = htmlBody
	return nil
}

type fakeHasher struct{}

func (fakeHasher) Hash(password string) (string, error) { return "hashed:" + password, nil }
func (fakeHasher) Verify(hash, password string) error    { return nil }

func TestServiceInviteValidationAndEmail(t *testing.T) {
	store := &fakeStore{}
	emailer := &fakeEmailer{}
	tokens, err := auth.NewTokenManager("access-secret-1234567890123456", "refresh-secret-1234567890123456", 15*time.Minute, 24*time.Hour)
	if err != nil {
		t.Fatalf("NewTokenManager failed: %v", err)
	}
	svc := NewService(store, emailer, fakeHasher{}, tokens, "http://localhost:5173", 7*24*time.Hour)

	wsID := uuid.New()
	userID := uuid.New()
	adminActor := workspaces.Actor{
		UserID:           userID,
		WorkspaceID:      wsID,
		MembershipID:     uuid.New(),
		WorkspaceRole:    workspaces.RoleAdmin,
		MembershipStatus: workspaces.MembershipActive,
		WorkspaceStatus:  workspaces.StatusActive,
		WorkspaceType:    workspaces.TypeCompany,
	}

	t.Run("invalid email", func(t *testing.T) {
		_, err := svc.Invite(context.Background(), adminActor, CreateInput{Email: "invalid-email", Role: workspaces.RoleManager})
		if !errors.Is(err, ErrInvalidEmail) {
			t.Errorf("got %v, want %v", err, ErrInvalidEmail)
		}
	})

	t.Run("invalid role", func(t *testing.T) {
		_, err := svc.Invite(context.Background(), adminActor, CreateInput{Email: "valid@example.com", Role: "invalid-role"})
		if !errors.Is(err, ErrInvalidRole) {
			t.Errorf("got %v, want %v", err, ErrInvalidRole)
		}
	})

	t.Run("admin cannot assign owner", func(t *testing.T) {
		_, err := svc.Invite(context.Background(), adminActor, CreateInput{Email: "valid@example.com", Role: workspaces.RoleOwner})
		if !errors.Is(err, workspaces.ErrForbidden) {
			t.Errorf("got %v, want %v", err, workspaces.ErrForbidden)
		}
	})

	t.Run("successful manager invitation", func(t *testing.T) {
		inv, err := svc.Invite(context.Background(), adminActor, CreateInput{Email: "manager@example.com", Role: workspaces.RoleManager})
		if err != nil {
			t.Fatalf("Invite failed: %v", err)
		}
		if inv.Email != "manager@example.com" {
			t.Errorf("email = %q, want manager@example.com", inv.Email)
		}
		if inv.Role != workspaces.RoleManager {
			t.Errorf("role = %q, want manager", inv.Role)
		}
		if emailer.sentTo != "manager@example.com" {
			t.Errorf("emailer.sentTo = %q, want manager@example.com", emailer.sentTo)
		}
	})
}

func TestServiceRegisterAndAccept(t *testing.T) {
	store := &fakeStore{}
	tokens, err := auth.NewTokenManager("access-secret-1234567890123456", "refresh-secret-1234567890123456", 15*time.Minute, 24*time.Hour)
	if err != nil {
		t.Fatalf("NewTokenManager failed: %v", err)
	}
	svc := NewService(store, &fakeEmailer{}, fakeHasher{}, tokens, "http://localhost:5173", 7*24*time.Hour)

	wsID := uuid.New()
	userID := uuid.New()
	store.registeredUser = auth.User{
		ID:           userID,
		Email:        "invited@example.com",
		Role:         auth.RoleManager,
		PlatformRole: auth.PlatformRoleUser,
		Status:       "active",
	}
	store.registerWsID = wsID

	t.Run("short password rejected", func(t *testing.T) {
		_, _, _, err := svc.RegisterAndAccept(context.Background(), "token123", "short")
		if err == nil {
			t.Error("expected error for short password, got nil")
		}
	})

	t.Run("valid registration issues tokens and joins workspace", func(t *testing.T) {
		pair, pubUser, targetWsID, err := svc.RegisterAndAccept(context.Background(), "valid-token", "securePassword123")
		if err != nil {
			t.Fatalf("RegisterAndAccept failed: %v", err)
		}
		if pair.AccessToken == "" || pair.RefreshToken == "" {
			t.Errorf("expected valid token pair, got %+v", pair)
		}
		if pubUser.Email != "invited@example.com" {
			t.Errorf("user email = %q, want invited@example.com", pubUser.Email)
		}
		if targetWsID != wsID {
			t.Errorf("targetWsID = %v, want %v", targetWsID, wsID)
		}
	})
}
