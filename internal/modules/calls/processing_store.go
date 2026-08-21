package calls

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrCallStatusChanged = errors.New("call status changed concurrently")

// CallProcessingStore is the worker-facing persistence boundary for call state.
// Transition checks both the domain transition table and the persisted prior state.
type CallProcessingStore interface {
	Get(ctx context.Context, callID uuid.UUID) (Call, error)
	Transition(ctx context.Context, callID uuid.UUID, from, to Status, errorMessage *string) error
}

// Get loads one call without applying API visibility scope because workers are
// trusted internal consumers of a queued call ID.
func (s *PostgresStore) Get(ctx context.Context, callID uuid.UUID) (Call, error) {
	call, err := scanCall(s.pool.QueryRow(ctx, `
SELECT id, workspace_id, owner_user_id, uploaded_by_user_id, manager_id, status, original_filename, object_key, content_type, size_bytes,
       duration_seconds, error_message, created_at, updated_at
FROM calls
WHERE id = $1`, callID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Call{}, ErrCallNotFound
	}
	return call, err
}

func (s *PostgresStore) GetInWorkspace(ctx context.Context, workspaceID, callID uuid.UUID) (Call, error) {
	call, err := scanCall(s.pool.QueryRow(ctx, `
SELECT id, workspace_id, owner_user_id, uploaded_by_user_id, manager_id, status, original_filename, object_key, content_type, size_bytes,
       duration_seconds, error_message, created_at, updated_at
FROM calls WHERE id = $1 AND workspace_id = $2`, callID, workspaceID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Call{}, ErrCallNotFound
	}
	return call, err
}

// Transition is the sole worker path for changing status or clearing/storing a
// processing error. The expected prior state prevents duplicate deliveries from
// overwriting a concurrent worker's progression.
func (s *PostgresStore) Transition(ctx context.Context, callID uuid.UUID, from, to Status, errorMessage *string) error {
	if err := ValidateTransition(from, to); err != nil {
		return err
	}
	result, err := s.pool.Exec(ctx, `
UPDATE calls
SET status = $3, error_message = $4, updated_at = NOW()
WHERE id = $1 AND status = $2`, callID, from, to, errorMessage)
	if err != nil {
		return fmt.Errorf("update call status: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrCallStatusChanged
	}
	return nil
}

func (s *PostgresStore) TransitionInWorkspace(ctx context.Context, workspaceID, callID uuid.UUID, from, to Status, errorMessage *string) error {
	if err := ValidateTransition(from, to); err != nil {
		return err
	}
	result, err := s.pool.Exec(ctx, `
UPDATE calls SET status = $4, error_message = $5, updated_at = NOW()
WHERE id = $1 AND workspace_id = $2 AND status = $3`, callID, workspaceID, from, to, errorMessage)
	if err != nil {
		return fmt.Errorf("update workspace call status: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrCallStatusChanged
	}
	return nil
}

var _ CallProcessingStore = (*PostgresStore)(nil)
