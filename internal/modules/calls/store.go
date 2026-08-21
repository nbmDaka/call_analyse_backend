package calls

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"call_analyse_backend/internal/modules/analysis"
	"call_analyse_backend/internal/modules/auth"
	"call_analyse_backend/internal/modules/scoring"
	"call_analyse_backend/internal/modules/transcription"
	"call_analyse_backend/internal/modules/workspaces"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore persists call metadata using parameterized pgx queries.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a call store backed by a PostgreSQL connection pool.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// Create inserts metadata only after the caller has stored the object.
func (s *PostgresStore) Create(ctx context.Context, call Call) (Call, error) {
	return scanCall(s.pool.QueryRow(ctx, `
INSERT INTO calls (id, workspace_id, owner_user_id, uploaded_by_user_id, manager_id, status, original_filename, object_key, content_type, size_bytes)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, workspace_id, owner_user_id, uploaded_by_user_id, manager_id, status, original_filename, object_key, content_type, size_bytes,
          duration_seconds, error_message, created_at, updated_at`,
		call.ID, call.WorkspaceID, call.OwnerUserID, call.UploadedByUserID, call.ManagerID, call.Status, call.OriginalFilename, call.ObjectKey, call.ContentType, call.SizeBytes))
}

// List applies actor scope in both total and row queries before applying pagination.
func (s *PostgresStore) List(ctx context.Context, actor Actor, page Page) (CallPage, error) {
	countQuery, countArgs, err := listCountQuery(actor)
	if err != nil {
		return CallPage{}, err
	}
	var total int
	if err := s.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return CallPage{}, err
	}

	query, args, err := listQuery(actor, page)
	if err != nil {
		return CallPage{}, err
	}
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return CallPage{}, err
	}
	defer rows.Close()

	pageResult := CallPage{Page: page.Number, PerPage: page.Size, Total: total}
	pageResult.TotalPages = (total + page.Size - 1) / page.Size
	for rows.Next() {
		call, err := scanCall(rows)
		if err != nil {
			return CallPage{}, err
		}
		pageResult.Calls = append(pageResult.Calls, call)
	}
	if err := rows.Err(); err != nil {
		return CallPage{}, err
	}
	return pageResult, nil
}

// Detail hides both absent and inaccessible calls behind ErrCallNotFound.
func (s *PostgresStore) Detail(ctx context.Context, actor Actor, callID uuid.UUID) (Call, error) {
	query, args, err := detailQuery(actor, callID)
	if err != nil {
		return Call{}, err
	}
	call, err := scanCall(s.pool.QueryRow(ctx, query, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return Call{}, ErrCallNotFound
	}
	return call, err
}

// FullDetail returns the stable API read-model envelope. Transcript, analysis,
// and score enrichment is intentionally nullable until their independent
// checkpoints are present.
func (s *PostgresStore) FullDetail(ctx context.Context, actor Actor, callID uuid.UUID) (CallDetail, error) {
	call, err := s.Detail(ctx, actor, callID)
	if err != nil {
		return CallDetail{}, err
	}
	var manager auth.PublicUser
	if err := s.pool.QueryRow(ctx, `SELECT id, email, platform_role, status, role, supervisor_id, email_verified_at IS NOT NULL, created_at, updated_at FROM users WHERE id = $1`, call.OwnerUserID).Scan(&manager.ID, &manager.Email, &manager.PlatformRole, &manager.Status, &manager.Role, &manager.SupervisorID, &manager.EmailVerified, &manager.CreatedAt, &manager.UpdatedAt); err != nil {
		return CallDetail{}, fmt.Errorf("load call manager: %w", err)
	}
	transcript, transcriptExists, err := transcription.NewPostgresStore(s.pool).GetInWorkspace(ctx, actor.WorkspaceID, call.ID)
	if err != nil {
		return CallDetail{}, err
	}
	result, score, analysisExists, err := analysis.NewPostgresStore(s.pool).GetInWorkspace(ctx, actor.WorkspaceID, call.ID)
	if err != nil {
		return CallDetail{}, err
	}
	var transcriptPtr *transcription.Transcript
	if transcriptExists {
		transcriptPtr = &transcript
	}
	var analysisPtr *analysis.Analysis
	var scorePtr *scoring.Score
	if analysisExists {
		analysisPtr, scorePtr = &result, &score
	}
	return CallDetail{
		Call:       call,
		Manager:    &manager,
		Audio:      AudioMetadata{Filename: call.OriginalFilename, ContentType: call.ContentType, SizeBytes: call.SizeBytes},
		Transcript: transcriptPtr,
		Analysis:   analysisPtr,
		Score:      scorePtr,
	}, nil
}

func (s *PostgresStore) Delete(ctx context.Context, callID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM calls WHERE id = $1`, callID)
	return err
}

func (s *PostgresStore) DeleteInWorkspace(ctx context.Context, workspaceID, callID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM calls WHERE id = $1 AND workspace_id = $2`, callID, workspaceID)
	return err
}

func listCountQuery(actor Actor) (string, []any, error) {
	where, args, err := listScope(actor)
	if err != nil {
		return "", nil, err
	}
	return "SELECT COUNT(*) FROM calls c WHERE " + where, args, nil
}

func listQuery(actor Actor, page Page) (string, []any, error) {
	where, args, err := listScope(actor)
	if err != nil {
		return "", nil, err
	}
	limitPlaceholder := len(args) + 1
	offsetPlaceholder := limitPlaceholder + 1
	query := fmt.Sprintf(`
SELECT c.id, c.workspace_id, c.owner_user_id, c.uploaded_by_user_id, c.manager_id, c.status, c.original_filename, c.object_key, c.content_type, c.size_bytes,
       c.duration_seconds, c.error_message, c.created_at, c.updated_at
FROM calls c
WHERE %s
ORDER BY c.created_at DESC
LIMIT $%d OFFSET $%d`, where, limitPlaceholder, offsetPlaceholder)
	args = append(args, page.Size, (page.Number-1)*page.Size)
	return query, args, nil
}

func detailQuery(actor Actor, callID uuid.UUID) (string, []any, error) {
	where, args, err := detailScope(actor)
	if err != nil {
		return "", nil, err
	}
	query := `
SELECT c.id, c.workspace_id, c.owner_user_id, c.uploaded_by_user_id, c.manager_id, c.status, c.original_filename, c.object_key, c.content_type, c.size_bytes,
       c.duration_seconds, c.error_message, c.created_at, c.updated_at
FROM calls c
WHERE c.id = $1 AND ` + where
	return query, append([]any{callID}, args...), nil
}

func listScope(actor Actor) (string, []any, error) {
	if actor.WorkspaceID == uuid.Nil || actor.UserID == uuid.Nil || actor.MembershipID == uuid.Nil {
		return "", nil, ErrInvalidActor
	}
	switch actor.WorkspaceRole {
	case workspaces.RoleOwner, workspaces.RoleAdmin:
		return "c.workspace_id = $1", []any{actor.WorkspaceID}, nil
	case workspaces.RoleManager:
		return "c.workspace_id = $1 AND c.owner_user_id = $2", []any{actor.WorkspaceID, actor.UserID}, nil
	case workspaces.RoleSupervisor:
		return `c.workspace_id = $1 AND (c.owner_user_id = $2 OR c.owner_user_id IN (
SELECT managed.user_id FROM workspace_memberships managed
WHERE managed.workspace_id = $1 AND managed.supervisor_membership_id = $3
  AND managed.role = 'manager' AND managed.status = 'active'))`, []any{actor.WorkspaceID, actor.UserID, actor.MembershipID}, nil
	default:
		return "", nil, ErrInvalidActor
	}
}

func detailScope(actor Actor) (string, []any, error) {
	where, args, err := listScope(actor)
	if err != nil {
		return "", nil, err
	}
	for i := len(args); i >= 1; i-- {
		where = strings.ReplaceAll(where, fmt.Sprintf("$%d", i), fmt.Sprintf("$%d", i+1))
	}
	return where, args, nil
}

type callRowScanner interface {
	Scan(dest ...any) error
}

func scanCall(row callRowScanner) (Call, error) {
	var call Call
	var status string
	err := row.Scan(
		&call.ID,
		&call.WorkspaceID,
		&call.OwnerUserID,
		&call.UploadedByUserID,
		&call.ManagerID,
		&status,
		&call.OriginalFilename,
		&call.ObjectKey,
		&call.ContentType,
		&call.SizeBytes,
		&call.DurationSeconds,
		&call.ErrorMessage,
		&call.CreatedAt,
		&call.UpdatedAt,
	)
	if err != nil {
		return Call{}, err
	}
	call.Status = Status(strings.ToLower(status))
	return call, nil
}

var _ CallStore = (*PostgresStore)(nil)
