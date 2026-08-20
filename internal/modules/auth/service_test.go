package auth

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestServiceRefreshRotatesAndRevokesPreviousToken(t *testing.T) {
	ctx := context.Background()
	verifiedAt := time.Now().UTC()
	user := User{ID: uuid.New(), Email: "manager@example.com", Role: RoleManager, EmailVerifiedAt: &verifiedAt}
	hasher := NewPasswordHasher()
	passwordHash, err := hasher.Hash("password")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	user.PasswordHash = passwordHash
	tokens := mustNewTokenManager(t, time.Hour, 24*time.Hour)
	users := &fakeUserStore{byEmail: map[string]User{user.Email: user}, byID: map[uuid.UUID]User{user.ID: user}}
	refreshes := &fakeRefreshStore{tokens: map[string]RefreshToken{}}
	service := NewService(users, refreshes, hasher, tokens)

	login, err := service.Login(ctx, user.Email, "password")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	rotated, err := service.Refresh(ctx, login.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if rotated.RefreshToken == login.RefreshToken {
		t.Fatal("Refresh() returned the previous refresh token")
	}
	old := refreshes.tokens[tokens.HashRefreshToken(login.RefreshToken)]
	if old.RevokedAt == nil {
		t.Fatal("Refresh() did not revoke the previous refresh token")
	}
	if _, err := service.Refresh(ctx, login.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("Refresh() with rotated token error = %v, want ErrInvalidRefreshToken", err)
	}
}

func TestServiceBootstrapAdminDoesNotOverwriteExistingUser(t *testing.T) {
	ctx := context.Background()
	hasher := NewPasswordHasher()
	existingHash, err := hasher.Hash("existing-password")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	existing := User{ID: uuid.New(), Email: "admin@example.com", PasswordHash: existingHash, Role: RoleManager}
	users := &fakeUserStore{byEmail: map[string]User{existing.Email: existing}, byID: map[uuid.UUID]User{existing.ID: existing}}
	service := NewService(users, &fakeRefreshStore{tokens: map[string]RefreshToken{}}, hasher, mustNewTokenManager(t, time.Hour, 24*time.Hour))

	got, created, err := service.BootstrapAdmin(ctx, existing.Email, "new-password")
	if err != nil {
		t.Fatalf("BootstrapAdmin() error = %v", err)
	}
	if created {
		t.Fatal("BootstrapAdmin() created = true, want false for existing user")
	}
	requirePublicUser(t, got)
	if got.ID != existing.ID || got.Email != existing.Email || got.Role != existing.Role {
		t.Fatalf("BootstrapAdmin() user = %#v, want public view of %#v", got, existing)
	}
	if err := hasher.Verify(users.byEmail[existing.Email].PasswordHash, "existing-password"); err != nil {
		t.Fatalf("BootstrapAdmin() overwrote the existing password hash: %v", err)
	}
}

func TestServiceLogoutRevokesRefreshSession(t *testing.T) {
	ctx := context.Background()
	user, service, tokens, refreshes := authenticatedServiceFixture(t)

	login, err := service.Login(ctx, user.Email, "password")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if err := service.Logout(ctx, login.RefreshToken); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if refreshes.tokens[tokens.HashRefreshToken(login.RefreshToken)].RevokedAt == nil {
		t.Fatal("Logout() did not revoke the active refresh session")
	}
}

func TestServiceMeReturnsAuthenticatedUser(t *testing.T) {
	ctx := context.Background()
	user, service, _, _ := authenticatedServiceFixture(t)

	got, err := service.Me(ctx, user.ID)
	if err != nil {
		t.Fatalf("Me() error = %v", err)
	}
	requirePublicUser(t, got)
	if got.ID != user.ID || got.Email != user.Email || got.Role != user.Role {
		t.Fatalf("Me() = %#v, want authenticated user %#v", got, user)
	}
}

func TestPublicUserDoesNotExposePasswordHash(t *testing.T) {
	if _, found := reflect.TypeOf(PublicUser{}).FieldByName("PasswordHash"); found {
		t.Fatal("PublicUser exposes PasswordHash")
	}
}

func requirePublicUser(t *testing.T, user PublicUser) {
	t.Helper()
}

func TestServiceRegisterCreatesNewUserAndReturnsTokens(t *testing.T) {
	ctx := context.Background()
	hasher := NewPasswordHasher()
	tokens := mustNewTokenManager(t, time.Hour, 24*time.Hour)
	users := &fakeUserStore{byEmail: map[string]User{}, byID: map[uuid.UUID]User{}}
	refreshes := &fakeRefreshStore{tokens: map[string]RefreshToken{}}
	service := NewService(users, refreshes, hasher, tokens)

	if err := service.Register(ctx, "newuser@example.com", "secret123"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	createdUser, exists := users.byEmail["newuser@example.com"]
	if !exists {
		t.Fatalf("User was not saved in store")
	}
	if createdUser.Role != RoleManager {
		t.Fatalf("User role = %v, want manager", createdUser.Role)
	}

	// Duplicate registration attempt
	if err := service.Register(ctx, "newuser@example.com", "secret123"); !errors.Is(err, ErrEmailAlreadyExists) {
		t.Fatalf("Register() duplicate error = %v, want ErrEmailAlreadyExists", err)
	}
}

func authenticatedServiceFixture(t *testing.T) (User, Service, TokenManager, *fakeRefreshStore) {
	t.Helper()
	verifiedAt := time.Now().UTC()
	user := User{ID: uuid.New(), Email: "manager@example.com", Role: RoleManager, EmailVerifiedAt: &verifiedAt}
	hasher := NewPasswordHasher()
	passwordHash, err := hasher.Hash("password")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	user.PasswordHash = passwordHash
	tokens := mustNewTokenManager(t, time.Hour, 24*time.Hour)
	users := &fakeUserStore{byEmail: map[string]User{user.Email: user}, byID: map[uuid.UUID]User{user.ID: user}}
	refreshes := &fakeRefreshStore{tokens: map[string]RefreshToken{}}
	return user, NewService(users, refreshes, hasher, tokens), tokens, refreshes
}

type fakeUserStore struct {
	byEmail map[string]User
	byID    map[uuid.UUID]User
}

func (s *fakeUserStore) FindByEmail(_ context.Context, email string) (User, error) {
	user, ok := s.byEmail[email]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return user, nil
}

func (s *fakeUserStore) FindByID(_ context.Context, id uuid.UUID) (User, error) {
	user, ok := s.byID[id]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return user, nil
}

func (s *fakeUserStore) Create(_ context.Context, user User) (User, error) {
	if _, exists := s.byEmail[user.Email]; exists {
		return User{}, ErrEmailAlreadyExists
	}
	s.byEmail[user.Email] = user
	s.byID[user.ID] = user
	return user, nil
}

func (s *fakeUserStore) MarkEmailVerified(_ context.Context, id uuid.UUID) error {
	user, ok := s.byID[id]
	if !ok {
		return ErrUserNotFound
	}
	now := time.Now().UTC()
	user.EmailVerifiedAt = &now
	s.byID[id] = user
	s.byEmail[user.Email] = user
	return nil
}

func (s *fakeUserStore) UpdatePassword(_ context.Context, id uuid.UUID, passwordHash string) error {
	user, ok := s.byID[id]
	if !ok {
		return ErrUserNotFound
	}
	user.PasswordHash = passwordHash
	s.byID[id] = user
	s.byEmail[user.Email] = user
	return nil
}

type fakeRefreshStore struct {
	tokens map[string]RefreshToken
}

func (s *fakeRefreshStore) Store(_ context.Context, token RefreshToken) error {
	s.tokens[token.TokenHash] = token
	return nil
}

func (s *fakeRefreshStore) FindActive(_ context.Context, hash string) (RefreshToken, error) {
	token, ok := s.tokens[hash]
	if !ok || token.RevokedAt != nil || !token.ExpiresAt.After(time.Now()) {
		return RefreshToken{}, ErrRefreshTokenNotFound
	}
	return token, nil
}

func (s *fakeRefreshStore) Rotate(_ context.Context, oldHash string, replacement RefreshToken) error {
	old, err := s.FindActive(context.Background(), oldHash)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	old.RevokedAt = &now
	s.tokens[oldHash] = old
	s.tokens[replacement.TokenHash] = replacement
	return nil
}

func (s *fakeRefreshStore) Revoke(_ context.Context, hash string) error {
	token, ok := s.tokens[hash]
	if !ok || token.RevokedAt != nil {
		return ErrRefreshTokenNotFound
	}
	now := time.Now().UTC()
	token.RevokedAt = &now
	s.tokens[hash] = token
	return nil
}

func (s *fakeRefreshStore) RevokeAllForUser(_ context.Context, userID uuid.UUID) error {
	now := time.Now().UTC()
	for hash, token := range s.tokens {
		if token.UserID == userID && token.RevokedAt == nil {
			token.RevokedAt = &now
			s.tokens[hash] = token
		}
	}
	return nil
}
