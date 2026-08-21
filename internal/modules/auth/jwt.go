package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type tokenManager struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

// ValidateTokenConfiguration enforces the access and refresh token security policy.
func ValidateTokenConfiguration(accessSecret, refreshSecret string, accessTTL, refreshTTL time.Duration) error {
	if strings.TrimSpace(accessSecret) == "" || strings.TrimSpace(refreshSecret) == "" {
		return fmt.Errorf("JWT access and refresh secrets are required")
	}
	if hmac.Equal([]byte(accessSecret), []byte(refreshSecret)) {
		return fmt.Errorf("JWT access and refresh secrets must differ")
	}
	if accessTTL <= 0 || accessTTL > MaxAccessTokenTTL {
		return fmt.Errorf("JWT access TTL must be greater than zero and no more than %s", MaxAccessTokenTTL)
	}
	if refreshTTL <= accessTTL {
		return fmt.Errorf("JWT refresh TTL must be longer than access TTL")
	}
	return nil
}

// NewTokenManager creates a manager with distinct signing secrets for each token type.
func NewTokenManager(accessSecret, refreshSecret string, accessTTL, refreshTTL time.Duration) (TokenManager, error) {
	if err := ValidateTokenConfiguration(accessSecret, refreshSecret, accessTTL, refreshTTL); err != nil {
		return nil, err
	}
	return tokenManager{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
	}, nil
}

func (m tokenManager) IssueAccess(user User) (string, error) {
	return m.issue(user, accessTokenType, m.accessSecret, m.accessTTL)
}

func (m tokenManager) IssueRefresh(user User) (string, error) {
	return m.issue(user, refreshTokenType, m.refreshSecret, m.refreshTTL)
}

func (m tokenManager) ParseAccess(token string) (Claims, error) {
	return m.parse(token, accessTokenType, m.accessSecret)
}

func (m tokenManager) ParseRefresh(token string) (Claims, error) {
	return m.parse(token, refreshTokenType, m.refreshSecret)
}

func (m tokenManager) HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (m tokenManager) VerifyRefreshTokenHash(hash, token string) bool {
	return hmac.Equal([]byte(hash), []byte(m.HashRefreshToken(token)))
}

func (m tokenManager) issue(user User, tokenType string, secret []byte, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	platformRole := user.PlatformRole
	if platformRole == "" {
		platformRole = PlatformRoleUser
	}
	claims := Claims{
		UserID:       user.ID.String(),
		PlatformRole: platformRole,
		Role:         user.Role,
		TokenType:    tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tokenIssuer,
			Subject:   user.ID.String(),
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

func (m tokenManager) parse(raw, expectedType string, secret []byte) (Claims, error) {
	claims := Claims{}
	token, err := jwt.ParseWithClaims(raw, &claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(tokenIssuer), jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		return Claims{}, ErrInvalidRefreshToken
	}
	if claims.TokenType != expectedType || claims.UserID == "" || claims.Subject != claims.UserID {
		return Claims{}, ErrInvalidRefreshToken
	}
	if claims.PlatformRole == "" {
		claims.PlatformRole = PlatformRoleUser
	}
	if claims.PlatformRole != PlatformRoleUser && claims.PlatformRole != PlatformRoleSuperAdmin {
		return Claims{}, ErrInvalidRefreshToken
	}
	return claims, nil
}
