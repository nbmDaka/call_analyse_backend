package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestNewTokenManagerRejectsMissingOrEqualSecrets(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		accessSecret  string
		refreshSecret string
	}{
		{name: "missing access", accessSecret: "", refreshSecret: "refresh-secret"},
		{name: "missing refresh", accessSecret: "access-secret", refreshSecret: ""},
		{name: "same secrets", accessSecret: "shared-secret", refreshSecret: "shared-secret"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := NewTokenManager(testCase.accessSecret, testCase.refreshSecret, 15*time.Minute, time.Hour); err == nil {
				t.Fatal("NewTokenManager() error = nil, want secret validation error")
			}
		})
	}
}

func TestNewTokenManagerEnforcesAccessAndRefreshTTLs(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		accessTTL  time.Duration
		refreshTTL time.Duration
	}{
		{name: "access exceeds one hour", accessTTL: time.Hour + time.Second, refreshTTL: 24 * time.Hour},
		{name: "refresh equals access", accessTTL: 30 * time.Minute, refreshTTL: 30 * time.Minute},
		{name: "refresh is shorter", accessTTL: 30 * time.Minute, refreshTTL: 29 * time.Minute},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := NewTokenManager("access-secret", "refresh-secret", testCase.accessTTL, testCase.refreshTTL); err == nil {
				t.Fatal("NewTokenManager() error = nil, want TTL validation error")
			}
		})
	}
}

func TestTokenManagerRejectsExpiredAccessToken(t *testing.T) {
	manager := mustNewTokenManager(t, 15*time.Minute, 24*time.Hour)
	userID := uuid.New()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID:    userID.String(),
		Role:      RoleManager,
		TokenType: accessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tokenIssuer,
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Minute)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		},
	}).SignedString([]byte("access-secret"))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	if _, err := manager.ParseAccess(token); err == nil {
		t.Fatal("ParseAccess() error = nil, want expired token rejection")
	}
}

func TestTokenManagerRejectsAccessTokenSignedWithWrongSecret(t *testing.T) {
	issuer, err := NewTokenManager("wrong-access-secret", "refresh-secret", time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatalf("NewTokenManager() issuer error = %v", err)
	}
	verifier := mustNewTokenManager(t, time.Hour, 24*time.Hour)

	token, err := issuer.IssueAccess(User{ID: uuid.New(), Role: RoleManager})
	if err != nil {
		t.Fatalf("IssueAccess() error = %v", err)
	}
	if _, err := verifier.ParseAccess(token); err == nil {
		t.Fatal("ParseAccess() error = nil, want wrong-secret rejection")
	}
}

func TestTokenManagerRejectsRefreshTokenAsAccessToken(t *testing.T) {
	manager := mustNewTokenManager(t, time.Hour, 24*time.Hour)

	refresh, err := manager.IssueRefresh(User{ID: uuid.New(), Role: RoleManager})
	if err != nil {
		t.Fatalf("IssueRefresh() error = %v", err)
	}
	if _, err := manager.ParseAccess(refresh); err == nil {
		t.Fatal("ParseAccess() error = nil, want refresh token rejection")
	}
	if _, err := manager.ParseAccess(manager.HashRefreshToken(refresh)); err == nil {
		t.Fatal("ParseAccess() error = nil, want refresh token hash rejection")
	}
}

func TestTokenManagerRejectsRefreshTypeSignedWithAccessSecret(t *testing.T) {
	manager := mustNewTokenManager(t, time.Hour, 24*time.Hour)
	userID := uuid.New()
	refreshTypeToken := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID:    userID.String(),
		Role:      RoleManager,
		TokenType: refreshTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tokenIssuer,
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	raw, err := refreshTypeToken.SignedString([]byte("access-secret"))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	if _, err := manager.ParseAccess(raw); err == nil {
		t.Fatal("ParseAccess() error = nil, want token_type rejection with matching signature")
	}
}

func TestTokenManagerRefreshHashComparisonRejectsDifferentToken(t *testing.T) {
	manager := mustNewTokenManager(t, time.Hour, 24*time.Hour)

	hash := manager.HashRefreshToken("refresh-token-one")
	if !manager.VerifyRefreshTokenHash(hash, "refresh-token-one") {
		t.Fatal("VerifyRefreshTokenHash() = false, want true for matching token")
	}
	if manager.VerifyRefreshTokenHash(hash, "refresh-token-two") {
		t.Fatal("VerifyRefreshTokenHash() = true, want false for different token")
	}
}

func mustNewTokenManager(t *testing.T, accessTTL, refreshTTL time.Duration) TokenManager {
	t.Helper()
	manager, err := NewTokenManager("access-secret", "refresh-secret", accessTTL, refreshTTL)
	if err != nil {
		t.Fatalf("NewTokenManager() error = %v", err)
	}
	return manager
}
