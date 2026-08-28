package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"call_analyse_backend/internal/modules/calls"
	"call_analyse_backend/internal/modules/workspaces"
	"call_analyse_backend/internal/transport/http/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s server) createCall(w http.ResponseWriter, r *http.Request) {
	if s.deps.Calls == nil {
		writeError(w, r, errors.New("call service is not configured"))
		return
	}
	actor, workspaceActor, ok := callActorFromRequest(r)
	if !ok {
		writeError(w, r, middleware.ErrUnauthenticated)
		return
	}
	if workspaceActor != nil && !workspaceActor.CanUpload() {
		if workspaceActor.WorkspaceStatus == workspaces.StatusSuspended {
			writeError(w, r, workspaces.ErrWorkspaceSuspended)
			return
		}
		writeError(w, r, calls.ErrInvalidActor)
		return
	}
	if workspaceActor == nil && actor.Role != "admin" && actor.Role != "manager" {
		writeError(w, r, calls.ErrInvalidActor)
		return
	}
	if s.deps.MaxUploadBytes <= 0 {
		writeInvalid(w, r, "upload limit is not configured")
		return
	}
	requestLimit := s.deps.MaxUploadBytes + 1024*1024
	r.Body = http.MaxBytesReader(w, r.Body, requestLimit)
	if err := r.ParseMultipartForm(requestLimit); err != nil {
		writeInvalid(w, r, "invalid multipart upload")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeInvalid(w, r, "file is required")
		return
	}
	defer file.Close()
	if err := calls.ValidateUpload(header.Filename, header.Header.Get("Content-Type"), header.Size, s.deps.MaxUploadBytes); err != nil {
		writeInvalid(w, r, err.Error())
		return
	}
	created, err := s.deps.Calls.Create(r.Context(), actor, calls.Upload{Filename: header.Filename, ContentType: header.Header.Get("Content-Type"), Size: header.Size, Reader: file})
	if err != nil {
		writeError(w, r, err)
		return
	}
	if workspaceActor != nil && s.deps.EnqueueWorkspaceCall == nil || workspaceActor == nil && s.deps.EnqueueCall == nil {
		if rollback, ok := s.deps.Calls.(interface {
			RollbackCreate(context.Context, calls.Call) error
		}); ok {
			_ = rollback.RollbackCreate(r.Context(), created)
		}
		writeError(w, r, errors.New("call queue is not configured"))
		return
	}
	var queueErr error
	if workspaceActor != nil {
		if queueable, ok := s.deps.Calls.(interface {
			QueueInWorkspace(context.Context, uuid.UUID, uuid.UUID) error
		}); ok {
			queueErr = queueable.QueueInWorkspace(r.Context(), created.WorkspaceID, created.ID)
		}
	} else if queueable, ok := s.deps.Calls.(interface {
		Queue(context.Context, uuid.UUID) error
	}); ok {
		queueErr = queueable.Queue(r.Context(), created.ID)
	}
	if queueErr != nil {
		if rollback, rollbackOK := s.deps.Calls.(interface {
			RollbackCreate(context.Context, calls.Call) error
		}); rollbackOK {
			_ = rollback.RollbackCreate(r.Context(), created)
		}
		writeError(w, r, queueErr)
		return
	}
	var enqueueErr error
	if workspaceActor != nil {
		enqueueErr = s.deps.EnqueueWorkspaceCall(r.Context(), created.WorkspaceID.String(), created.ID.String())
	} else {
		enqueueErr = s.deps.EnqueueCall(r.Context(), created.ID.String())
	}
	if enqueueErr != nil {
		if rollback, ok := s.deps.Calls.(interface {
			RollbackCreate(context.Context, calls.Call) error
		}); ok {
			_ = rollback.RollbackCreate(r.Context(), created)
		}
		writeError(w, r, enqueueErr)
		return
	}
	if created.Status != calls.StatusQueued {
		created.Status = calls.StatusQueued
	}
	writeJSON(w, http.StatusCreated, map[string]any{"call": created})
}

func (s server) listCalls(w http.ResponseWriter, r *http.Request) {
	if s.deps.Calls == nil {
		writeError(w, r, errors.New("call service is not configured"))
		return
	}
	actor, _, ok := callActorFromRequest(r)
	if !ok {
		writeError(w, r, middleware.ErrUnauthenticated)
		return
	}
	page, size := 1, 20
	var err error
	if value := r.URL.Query().Get("page"); value != "" {
		page, err = strconv.Atoi(value)
	}
	if err == nil {
		if value := r.URL.Query().Get("page_size"); value != "" {
			size, err = strconv.Atoi(value)
		}
	}
	if err != nil {
		writeInvalid(w, r, "invalid pagination")
		return
	}
	var managerID *uuid.UUID
	if value := r.URL.Query().Get("manager_id"); value != "" {
		if parsed, parseErr := uuid.Parse(value); parseErr == nil {
			managerID = &parsed
		} else {
			writeInvalid(w, r, "invalid manager_id")
			return
		}
	}
	result, err := s.deps.Calls.List(r.Context(), actor, calls.Page{Number: page, Size: size, ManagerID: managerID})
	if err != nil {
		if isPaginationError(err) {
			writeInvalid(w, r, "invalid pagination")
		} else {
			writeError(w, r, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s server) detailCall(w http.ResponseWriter, r *http.Request) {
	if s.deps.Calls == nil {
		writeError(w, r, errors.New("call service is not configured"))
		return
	}
	actorValue, _, ok := callActorFromRequest(r)
	if !ok {
		writeError(w, r, middleware.ErrUnauthenticated)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, calls.ErrCallNotFound)
		return
	}
	if detailed, ok := s.deps.Calls.(interface {
		FullDetail(context.Context, calls.Actor, uuid.UUID) (calls.CallDetail, error)
	}); ok {
		detail, err := detailed.FullDetail(r.Context(), actorValue, id)
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, detail)
		return
	}
	result, err := s.deps.Calls.Detail(r.Context(), actorValue, id)
	if err != nil {
		if errors.Is(err, calls.ErrCallNotFound) {
			writeError(w, r, calls.ErrCallNotFound)
		} else {
			writeError(w, r, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"call": result, "manager": map[string]any{"id": result.ManagerID}, "audio": map[string]any{"filename": result.OriginalFilename, "content_type": result.ContentType, "size_bytes": result.SizeBytes}, "transcript": nil, "analysis": nil, "score": nil})
}

func callActorFromRequest(r *http.Request) (calls.Actor, *workspaces.Actor, bool) {
	if workspaceActor, ok := middleware.WorkspaceActorFromContext(r.Context()); ok {
		return calls.ActorFromWorkspace(workspaceActor), &workspaceActor, true
	}
	identity, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		return calls.Actor{}, nil, false
	}
	return calls.Actor{ID: identity.ID, Role: identity.Role}, nil, true
}

func isPaginationError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "page number") || strings.Contains(message, "page size")
}
