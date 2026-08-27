package playbooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrPlaybookNotFound = errors.New("playbook not found")
)

// Store defines persistence operations for workspace playbooks.
type Store interface {
	List(ctx context.Context, workspaceID uuid.UUID) ([]Playbook, error)
	GetByID(ctx context.Context, workspaceID, id uuid.UUID) (Playbook, error)
	GetDefault(ctx context.Context, workspaceID uuid.UUID) (Playbook, error)
	Create(ctx context.Context, playbook Playbook) (Playbook, error)
	Update(ctx context.Context, playbook Playbook) (Playbook, error)
	Delete(ctx context.Context, workspaceID, id uuid.UUID) error
}

// PostgresStore implements Store using PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore returns a new PostgreSQL playbook store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// List returns all active playbooks belonging to the specified workspace.
func (s *PostgresStore) List(ctx context.Context, workspaceID uuid.UUID) ([]Playbook, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id, workspace_id, name, COALESCE(description, ''), is_default, criteria, created_at, updated_at, deleted_at
FROM playbooks
WHERE workspace_id = $1 AND deleted_at IS NULL
ORDER BY is_default DESC, created_at ASC`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list playbooks: %w", err)
	}
	defer rows.Close()

	var list []Playbook
	for rows.Next() {
		var p Playbook
		var criteriaJSON []byte
		if err := rows.Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.Description, &p.IsDefault, &criteriaJSON, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt); err != nil {
			return nil, fmt.Errorf("scan playbook: %w", err)
		}
		if len(criteriaJSON) > 0 {
			if err := json.Unmarshal(criteriaJSON, &p.Criteria); err != nil {
				return nil, fmt.Errorf("decode criteria: %w", err)
			}
		}
		list = append(list, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate playbooks: %w", err)
	}
	if list == nil {
		list = []Playbook{}
	}
	return list, nil
}

// GetByID returns one active playbook scoped to the specified workspace.
func (s *PostgresStore) GetByID(ctx context.Context, workspaceID, id uuid.UUID) (Playbook, error) {
	var p Playbook
	var criteriaJSON []byte
	err := s.pool.QueryRow(ctx, `
SELECT id, workspace_id, name, COALESCE(description, ''), is_default, criteria, created_at, updated_at, deleted_at
FROM playbooks
WHERE workspace_id = $1 AND id = $2 AND deleted_at IS NULL`, workspaceID, id).Scan(
		&p.ID, &p.WorkspaceID, &p.Name, &p.Description, &p.IsDefault, &criteriaJSON, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Playbook{}, ErrPlaybookNotFound
	}
	if err != nil {
		return Playbook{}, fmt.Errorf("get playbook by ID: %w", err)
	}
	if len(criteriaJSON) > 0 {
		if err := json.Unmarshal(criteriaJSON, &p.Criteria); err != nil {
			return Playbook{}, fmt.Errorf("decode criteria: %w", err)
		}
	}
	return p, nil
}

// GetDefault returns the default active playbook for the workspace or falls back to the earliest created active one.
func (s *PostgresStore) GetDefault(ctx context.Context, workspaceID uuid.UUID) (Playbook, error) {
	var p Playbook
	var criteriaJSON []byte
	err := s.pool.QueryRow(ctx, `
SELECT id, workspace_id, name, COALESCE(description, ''), is_default, criteria, created_at, updated_at, deleted_at
FROM playbooks
WHERE workspace_id = $1 AND deleted_at IS NULL
ORDER BY is_default DESC, created_at ASC
LIMIT 1`, workspaceID).Scan(
		&p.ID, &p.WorkspaceID, &p.Name, &p.Description, &p.IsDefault, &criteriaJSON, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Playbook{}, ErrPlaybookNotFound
	}
	if err != nil {
		return Playbook{}, fmt.Errorf("get default playbook: %w", err)
	}
	if len(criteriaJSON) > 0 {
		if err := json.Unmarshal(criteriaJSON, &p.Criteria); err != nil {
			return Playbook{}, fmt.Errorf("decode criteria: %w", err)
		}
	}
	return p, nil
}

// Create inserts a new playbook. If is_default is true, previous default is unset among active playbooks.
func (s *PostgresStore) Create(ctx context.Context, playbook Playbook) (Playbook, error) {
	criteriaJSON, err := json.Marshal(playbook.Criteria)
	if err != nil {
		return Playbook{}, fmt.Errorf("encode criteria: %w", err)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Playbook{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if playbook.IsDefault {
		if _, err := tx.Exec(ctx, `UPDATE playbooks SET is_default = FALSE WHERE workspace_id = $1 AND deleted_at IS NULL`, playbook.WorkspaceID); err != nil {
			return Playbook{}, fmt.Errorf("unset existing default: %w", err)
		}
	}

	var created Playbook
	var scannedCriteria []byte
	err = tx.QueryRow(ctx, `
INSERT INTO playbooks (workspace_id, name, description, is_default, criteria)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, workspace_id, name, COALESCE(description, ''), is_default, criteria, created_at, updated_at, deleted_at`,
		playbook.WorkspaceID, playbook.Name, playbook.Description, playbook.IsDefault, criteriaJSON,
	).Scan(&created.ID, &created.WorkspaceID, &created.Name, &created.Description, &created.IsDefault, &scannedCriteria, &created.CreatedAt, &created.UpdatedAt, &created.DeletedAt)
	if err != nil {
		return Playbook{}, fmt.Errorf("insert playbook: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Playbook{}, fmt.Errorf("commit transaction: %w", err)
	}
	if len(scannedCriteria) > 0 {
		_ = json.Unmarshal(scannedCriteria, &created.Criteria)
	}
	return created, nil
}

// Update modifies an existing playbook.
func (s *PostgresStore) Update(ctx context.Context, playbook Playbook) (Playbook, error) {
	criteriaJSON, err := json.Marshal(playbook.Criteria)
	if err != nil {
		return Playbook{}, fmt.Errorf("encode criteria: %w", err)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Playbook{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if playbook.IsDefault {
		if _, err := tx.Exec(ctx, `UPDATE playbooks SET is_default = FALSE WHERE workspace_id = $1 AND id <> $2 AND deleted_at IS NULL`, playbook.WorkspaceID, playbook.ID); err != nil {
			return Playbook{}, fmt.Errorf("unset existing default: %w", err)
		}
	}

	var updated Playbook
	var scannedCriteria []byte
	err = tx.QueryRow(ctx, `
UPDATE playbooks
SET name = $1, description = $2, is_default = $3, criteria = $4, updated_at = NOW()
WHERE workspace_id = $5 AND id = $6 AND deleted_at IS NULL
RETURNING id, workspace_id, name, COALESCE(description, ''), is_default, criteria, created_at, updated_at, deleted_at`,
		playbook.Name, playbook.Description, playbook.IsDefault, criteriaJSON, playbook.WorkspaceID, playbook.ID,
	).Scan(&updated.ID, &updated.WorkspaceID, &updated.Name, &updated.Description, &updated.IsDefault, &scannedCriteria, &updated.CreatedAt, &updated.UpdatedAt, &updated.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Playbook{}, ErrPlaybookNotFound
	}
	if err != nil {
		return Playbook{}, fmt.Errorf("update playbook: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Playbook{}, fmt.Errorf("commit transaction: %w", err)
	}
	if len(scannedCriteria) > 0 {
		_ = json.Unmarshal(scannedCriteria, &updated.Criteria)
	}
	return updated, nil
}

// Delete logically deletes a playbook by setting deleted_at.
func (s *PostgresStore) Delete(ctx context.Context, workspaceID, id uuid.UUID) error {
	cmd, err := s.pool.Exec(ctx, `
UPDATE playbooks
SET deleted_at = NOW(), is_default = FALSE, updated_at = NOW()
WHERE workspace_id = $1 AND id = $2 AND deleted_at IS NULL`, workspaceID, id)
	if err != nil {
		return fmt.Errorf("delete playbook: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrPlaybookNotFound
	}
	return nil
}

var _ Store = (*PostgresStore)(nil)
