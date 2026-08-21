package platform

import (
	"context"
	"encoding/json"

	"call_analyse_backend/internal/modules/workspaces"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) CreateCompany(ctx context.Context, actorUserID, ownerUserID uuid.UUID, name string) (workspaces.Workspace, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return workspaces.Workspace{}, err
	}
	defer tx.Rollback(ctx)
	item := workspaces.Workspace{ID: uuid.New(), Name: name, Type: workspaces.TypeCompany, Status: workspaces.StatusActive, OwnerUserID: ownerUserID}
	err = tx.QueryRow(ctx, `INSERT INTO workspaces (id,name,type,status,owner_user_id) VALUES ($1,$2,'company','active',$3) RETURNING created_at,updated_at`, item.ID, item.Name, ownerUserID).Scan(&item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return workspaces.Workspace{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO workspace_memberships (id,workspace_id,user_id,role,status) VALUES ($1,$2,$3,'owner','active')`, uuid.New(), item.ID, ownerUserID); err != nil {
		return workspaces.Workspace{}, err
	}
	if err := insertAudit(ctx, tx, actorUserID, "platform.workspace.created", "workspace", item.ID, &item.ID, map[string]string{"type": "company"}); err != nil {
		return workspaces.Workspace{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return workspaces.Workspace{}, err
	}
	return item, nil
}

func (s *PostgresStore) ListWorkspaces(ctx context.Context, workspaceType *workspaces.Type) ([]workspaces.Workspace, error) {
	query := `SELECT id, name, type, status, owner_user_id, created_at, updated_at FROM workspaces`
	args := []any{}
	if workspaceType != nil {
		query += " WHERE type = $1"
		args = append(args, *workspaceType)
	}
	query += " ORDER BY created_at DESC, id"
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]workspaces.Workspace, 0)
	for rows.Next() {
		var item workspaces.Workspace
		if err := rows.Scan(&item.ID, &item.Name, &item.Type, &item.Status, &item.OwnerUserID, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *PostgresStore) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, email, platform_role, status FROM users ORDER BY created_at DESC, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]User, 0)
	for rows.Next() {
		var item User
		if err := rows.Scan(&item.ID, &item.Email, &item.PlatformRole, &item.Status); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *PostgresStore) SetWorkspaceStatus(ctx context.Context, actorUserID, workspaceID uuid.UUID, status workspaces.Status) (workspaces.Workspace, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return workspaces.Workspace{}, err
	}
	defer tx.Rollback(ctx)
	var item workspaces.Workspace
	err = tx.QueryRow(ctx, `UPDATE workspaces SET status = $2, updated_at = NOW() WHERE id = $1 RETURNING id, name, type, status, owner_user_id, created_at, updated_at`, workspaceID, status).Scan(&item.ID, &item.Name, &item.Type, &item.Status, &item.OwnerUserID, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return workspaces.Workspace{}, err
	}
	if err := insertAudit(ctx, tx, actorUserID, "platform.workspace.status_changed", "workspace", workspaceID, &workspaceID, map[string]string{"status": string(status)}); err != nil {
		return workspaces.Workspace{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return workspaces.Workspace{}, err
	}
	return item, nil
}

func (s *PostgresStore) SetUserStatus(ctx context.Context, actorUserID, userID uuid.UUID, status string) (User, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)
	var item User
	err = tx.QueryRow(ctx, `UPDATE users SET status = $2, updated_at = NOW() WHERE id = $1 RETURNING id, email, platform_role, status`, userID, status).Scan(&item.ID, &item.Email, &item.PlatformRole, &item.Status)
	if err != nil {
		return User{}, err
	}
	if err := insertAudit(ctx, tx, actorUserID, "platform.user.status_changed", "user", userID, nil, map[string]string{"status": status}); err != nil {
		return User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return item, nil
}

func (s *PostgresStore) Metrics(ctx context.Context) (Metrics, error) {
	var result Metrics
	err := s.pool.QueryRow(ctx, `SELECT (SELECT COUNT(*) FROM users)::int, (SELECT COUNT(*) FROM workspaces)::int, (SELECT COUNT(*) FROM calls)::int`).Scan(&result.Users, &result.Workspaces, &result.Calls)
	return result, err
}

func insertAudit(ctx context.Context, tx pgx.Tx, actorUserID uuid.UUID, action, targetType string, targetID uuid.UUID, workspaceID *uuid.UUID, metadata any) error {
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_events (actor_user_id, action, target_type, target_id, workspace_id, metadata) VALUES ($1,$2,$3,$4,$5,$6)`, actorUserID, action, targetType, targetID, workspaceID, encoded)
	return err
}

var _ Store = (*PostgresStore)(nil)
