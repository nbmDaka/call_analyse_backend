package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"call_analyse_backend/internal/auth"
	"call_analyse_backend/internal/calls"
	"call_analyse_backend/internal/middleware"
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
	case errors.Is(err, calls.ErrCallNotFound):
		return apiError{http.StatusNotFound, "CALL_NOT_FOUND", "call not found"}
	case errors.Is(err, calls.ErrInvalidActor):
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
