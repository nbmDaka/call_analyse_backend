// Package middleware contains HTTP authentication and authorization helpers.
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"call_analyse_backend/internal/modules/auth"

	"github.com/google/uuid"
)

type claimsContextKey struct{}
type actorContextKey struct{}

// ErrUnauthenticated indicates a missing, malformed, or invalid bearer identity.
var ErrUnauthenticated = errors.New("unauthenticated")

// Actor is the minimal authenticated identity handlers pass to scoped services.
type Actor struct {
	ID   uuid.UUID
	Role auth.Role
}

// Authenticate verifies a Bearer access token and attaches its claims to the request context.
func Authenticate(tokens auth.TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			parts := strings.Fields(r.Header.Get("Authorization"))
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			claims, err := tokens.ParseAccess(parts[1])
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsContextKey{}, claims)))
		})
	}
}

// ParseActor validates the request bearer token and converts its claims to an actor.
func ParseActor(r *http.Request, tokens auth.TokenManager) (Actor, error) {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return Actor{}, ErrUnauthenticated
	}
	claims, err := tokens.ParseAccess(parts[1])
	if err != nil {
		return Actor{}, ErrUnauthenticated
	}
	id, err := uuid.Parse(claims.UserID)
	if err != nil || !validRole(claims.Role) {
		return Actor{}, ErrUnauthenticated
	}
	return Actor{ID: id, Role: claims.Role}, nil
}

// WithActor adds a successfully validated actor to a request context.
func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorContextKey{}, actor)
}

// ActorFromContext returns the actor added after bearer-token validation.
func ActorFromContext(ctx context.Context) (Actor, bool) {
	actor, ok := ctx.Value(actorContextKey{}).(Actor)
	return actor, ok
}

// HasRole reports whether actorRole is included in the permitted roles.
func HasRole(actorRole auth.Role, allowed ...auth.Role) bool {
	for _, role := range allowed {
		if actorRole == role {
			return true
		}
	}
	return false
}

func validRole(role auth.Role) bool {
	return role == auth.RoleAdmin || role == auth.RoleSupervisor || role == auth.RoleManager
}

// ClaimsFromContext returns authentication claims added by Authenticate.
func ClaimsFromContext(ctx context.Context) (auth.Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey{}).(auth.Claims)
	return claims, ok
}
