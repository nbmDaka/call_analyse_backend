package workspaces

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store interface {
	ListForUser(context.Context, uuid.UUID) ([]AvailableWorkspace, error)
	GetForUser(context.Context, uuid.UUID, uuid.UUID) (AvailableWorkspace, error)
	CreateCompany(context.Context, uuid.UUID, string) (AvailableWorkspace, error)
	Rename(context.Context, uuid.UUID, string) (Workspace, error)
}

type ActorResolver interface {
	ResolveActor(context.Context, uuid.UUID, PlatformRole, uuid.UUID) (Actor, error)
}

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) ListForUser(ctx context.Context, userID uuid.UUID) ([]AvailableWorkspace, error) {
	rows, err := s.pool.Query(ctx, `
SELECT w.id, w.name, w.type, w.status, w.owner_user_id, w.created_at, w.updated_at,
       m.id, m.role, m.status
FROM workspace_memberships m
JOIN workspaces w ON w.id = m.workspace_id
JOIN users u ON u.id = m.user_id
WHERE m.user_id = $1 AND u.status = 'active'
ORDER BY CASE WHEN w.type = 'personal' THEN 0 ELSE 1 END, w.created_at, w.id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	defer rows.Close()
	result := make([]AvailableWorkspace, 0)
	for rows.Next() {
		item, err := scanAvailableWorkspace(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *PostgresStore) GetForUser(ctx context.Context, userID, workspaceID uuid.UUID) (AvailableWorkspace, error) {
	item, err := scanAvailableWorkspace(s.pool.QueryRow(ctx, `
SELECT w.id, w.name, w.type, w.status, w.owner_user_id, w.created_at, w.updated_at,
       m.id, m.role, m.status
FROM workspace_memberships m
JOIN workspaces w ON w.id = m.workspace_id
JOIN users u ON u.id = m.user_id
WHERE m.user_id = $1 AND m.workspace_id = $2 AND u.status = 'active'`, userID, workspaceID))
	if errors.Is(err, pgx.ErrNoRows) {
		return AvailableWorkspace{}, ErrWorkspaceNotFound
	}
	return item, err
}

func (s *PostgresStore) ResolveActor(ctx context.Context, userID uuid.UUID, platformRole PlatformRole, workspaceID uuid.UUID) (Actor, error) {
	var actor Actor
	actor.UserID, actor.WorkspaceID, actor.PlatformRole = userID, workspaceID, platformRole
	var userStatus string
	err := s.pool.QueryRow(ctx, `
SELECT m.id, m.role, m.status, w.status, w.type, u.status
FROM workspace_memberships m
JOIN workspaces w ON w.id = m.workspace_id
JOIN users u ON u.id = m.user_id
WHERE m.user_id = $1 AND m.workspace_id = $2`, userID, workspaceID).Scan(
		&actor.MembershipID, &actor.WorkspaceRole, &actor.MembershipStatus, &actor.WorkspaceStatus, &actor.WorkspaceType, &userStatus,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Actor{}, ErrWorkspaceNotFound
	}
	if err != nil {
		return Actor{}, fmt.Errorf("resolve workspace actor: %w", err)
	}
	if userStatus != "active" || actor.MembershipStatus != MembershipActive {
		return Actor{}, ErrMembershipDisabled
	}
	return actor, nil
}

func (s *PostgresStore) CreateCompany(ctx context.Context, ownerUserID uuid.UUID, name string) (AvailableWorkspace, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AvailableWorkspace{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	workspace := Workspace{ID: uuid.New(), Name: name, Type: TypeCompany, Status: StatusActive, OwnerUserID: ownerUserID}
	err = tx.QueryRow(ctx, `
INSERT INTO workspaces (id, name, type, status, owner_user_id)
VALUES ($1, $2, 'company', 'active', $3)
RETURNING created_at, updated_at`, workspace.ID, workspace.Name, workspace.OwnerUserID).Scan(&workspace.CreatedAt, &workspace.UpdatedAt)
	if err != nil {
		return AvailableWorkspace{}, fmt.Errorf("create company workspace: %w", err)
	}
	membership := Membership{ID: uuid.New(), WorkspaceID: workspace.ID, UserID: ownerUserID, Role: RoleOwner, Status: MembershipActive}
	err = tx.QueryRow(ctx, `
INSERT INTO workspace_memberships (id, workspace_id, user_id, role, status)
VALUES ($1, $2, $3, 'owner', 'active')
RETURNING created_at, updated_at`, membership.ID, workspace.ID, ownerUserID).Scan(&membership.CreatedAt, &membership.UpdatedAt)
	if err != nil {
		return AvailableWorkspace{}, fmt.Errorf("create owner membership: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return AvailableWorkspace{}, err
	}
	return AvailableWorkspace{Workspace: workspace, MembershipID: membership.ID, MembershipRole: membership.Role, MembershipStatus: membership.Status}, nil
}

func (s *PostgresStore) Rename(ctx context.Context, workspaceID uuid.UUID, name string) (Workspace, error) {
	var workspace Workspace
	err := s.pool.QueryRow(ctx, `
UPDATE workspaces SET name = $2, updated_at = NOW() WHERE id = $1
RETURNING id, name, type, status, owner_user_id, created_at, updated_at`, workspaceID, name).Scan(
		&workspace.ID, &workspace.Name, &workspace.Type, &workspace.Status, &workspace.OwnerUserID, &workspace.CreatedAt, &workspace.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Workspace{}, ErrWorkspaceNotFound
	}
	return workspace, err
}

type rowScanner interface{ Scan(...any) error }

func scanAvailableWorkspace(row rowScanner) (AvailableWorkspace, error) {
	var item AvailableWorkspace
	err := row.Scan(&item.ID, &item.Name, &item.Type, &item.Status, &item.OwnerUserID, &item.CreatedAt, &item.UpdatedAt,
		&item.MembershipID, &item.MembershipRole, &item.MembershipStatus)
	return item, err
}

var _ Store = (*PostgresStore)(nil)
var _ ActorResolver = (*PostgresStore)(nil)
