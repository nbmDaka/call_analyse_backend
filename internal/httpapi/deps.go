package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"call_analyse_backend/internal/auth"
	"call_analyse_backend/internal/calls"
	"call_analyse_backend/internal/dashboard"

	"github.com/google/uuid"
)

type authService interface {
	Login(context.Context, string, string) (auth.TokenPair, error)
	Refresh(context.Context, string) (auth.TokenPair, error)
	Logout(context.Context, string) error
	Me(context.Context, uuid.UUID) (auth.PublicUser, error)
}

type callsService interface {
	Create(context.Context, calls.Actor, calls.Upload) (calls.Call, error)
	List(context.Context, calls.Actor, calls.Page) (calls.CallPage, error)
	Detail(context.Context, calls.Actor, uuid.UUID) (calls.Call, error)
}

type dashboardService interface {
	Summary(context.Context, calls.Actor) (dashboard.Summary, error)
}

// Dependencies are the application boundaries required by the HTTP layer.
type Dependencies struct {
	Authentication authService
	Calls          callsService
	Dashboard      dashboardService
	Tokens         auth.TokenManager
	EnqueueCall    func(context.Context, string) error
	Ready          func(context.Context) error
	MaxUploadBytes int64
	RequestTimeout time.Duration
	Logger         *slog.Logger
}

type server struct {
	deps Dependencies
}

var _ http.Handler = http.HandlerFunc(nil)
