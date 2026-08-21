package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"call_analyse_backend/internal/modules/auth"
	"call_analyse_backend/internal/modules/calls"
	platformadmin "call_analyse_backend/internal/modules/platform"
	"call_analyse_backend/internal/modules/workspaces"
	"call_analyse_backend/internal/transport/http/middleware"
)

type apiError struct {
	Status  int
	Code    string
	Message string
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	e := mapError(err)
	requestID := middleware.RequestIDFromContext(r.Context())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.Status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":      map[string]string{"code": e.Code, "message": e.Message},
		"request_id": requestID,
	})
}

func mapError(err error) apiError {
	switch {
	case errors.Is(err, middleware.ErrUnauthenticated), errors.Is(err, auth.ErrInvalidCredentials), errors.Is(err, auth.ErrInvalidRefreshToken):
		return apiError{http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required"}
	case errors.Is(err, auth.ErrEmailNotVerified):
		return apiError{http.StatusForbidden, "EMAIL_NOT_VERIFIED", "email verification is required"}
	case errors.Is(err, auth.ErrUserSuspended):
		return apiError{http.StatusForbidden, "USER_SUSPENDED", "user is suspended"}
	case errors.Is(err, auth.ErrInvalidActionToken):
		return apiError{http.StatusBadRequest, "INVALID_TOKEN", "link is invalid or expired"}
	case errors.Is(err, auth.ErrEmailServiceUnavailable):
		return apiError{http.StatusServiceUnavailable, "EMAIL_SERVICE_UNAVAILABLE", "email service is unavailable"}
	case errors.Is(err, calls.ErrCallNotFound):
		return apiError{http.StatusNotFound, "CALL_NOT_FOUND", "call not found"}
	case errors.Is(err, workspaces.ErrWorkspaceNotFound), errors.Is(err, workspaces.ErrMembershipNotFound):
		return apiError{http.StatusNotFound, "NOT_FOUND", "resource not found"}
	case errors.Is(err, workspaces.ErrWorkspaceSuspended):
		return apiError{http.StatusForbidden, "WORKSPACE_SUSPENDED", "workspace is suspended"}
	case errors.Is(err, workspaces.ErrMembershipDisabled):
		return apiError{http.StatusForbidden, "MEMBERSHIP_DISABLED", "workspace membership is disabled"}
	case errors.Is(err, workspaces.ErrLastOwner):
		return apiError{http.StatusConflict, "LAST_OWNER", "workspace must retain an active owner"}
	case errors.Is(err, workspaces.ErrInvalidSupervisor):
		return apiError{http.StatusBadRequest, "INVALID_SUPERVISOR", "supervisor must be active in the same workspace"}
	case errors.Is(err, calls.ErrInvalidActor), errors.Is(err, workspaces.ErrForbidden), errors.Is(err, platformadmin.ErrForbidden):
		return apiError{http.StatusForbidden, "FORBIDDEN", "access denied"}
	default:
		return apiError{http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error"}
	}
}

func writeInvalid(w http.ResponseWriter, r *http.Request, message string) {
	e := apiError{http.StatusBadRequest, "INVALID_REQUEST", message}
	requestID := middleware.RequestIDFromContext(r.Context())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.Status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": e.Code, "message": e.Message}, "request_id": requestID})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
