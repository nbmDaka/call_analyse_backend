package memberships

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"call_analyse_backend/internal/modules/workspaces"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) List(ctx context.Context, actor workspaces.Actor) ([]workspaces.Membership, error) {
	where := "m.workspace_id = $1"
	args := []any{actor.WorkspaceID}
	switch actor.WorkspaceRole {
	case workspaces.RoleOwner, workspaces.RoleAdmin:
	case workspaces.RoleSupervisor:
		where += " AND (m.id = $2 OR m.supervisor_membership_id = $2)"
		args = append(args, actor.MembershipID)
	case workspaces.RoleManager:
		where += " AND m.id = $2"
		args = append(args, actor.MembershipID)
	default:
		return nil, workspaces.ErrForbidden
	}
	rows, err := s.pool.Query(ctx, `
SELECT m.id, m.workspace_id, m.user_id, u.email, m.role, m.status,
       m.supervisor_membership_id, m.created_at, m.updated_at
FROM workspace_memberships m
JOIN users u ON u.id = m.user_id
WHERE `+where+`
ORDER BY CASE m.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 WHEN 'supervisor' THEN 2 ELSE 3 END, u.email`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]workspaces.Membership, 0)
	for rows.Next() {
		member, err := scanMembership(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, member)
	}
	return result, rows.Err()
}

func (s *PostgresStore) Get(ctx context.Context, workspaceID, membershipID uuid.UUID) (workspaces.Membership, error) {
	member, err := scanMembership(s.pool.QueryRow(ctx, `
SELECT m.id, m.workspace_id, m.user_id, u.email, m.role, m.status,
       m.supervisor_membership_id, m.created_at, m.updated_at
FROM workspace_memberships m
JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = $1 AND m.id = $2`, workspaceID, membershipID))
	if errors.Is(err, pgx.ErrNoRows) {
		return workspaces.Membership{}, workspaces.ErrMembershipNotFound
	}
	return member, err
}

func (s *PostgresStore) CreateByEmail(ctx context.Context, workspaceID uuid.UUID, input CreateInput) (workspaces.Membership, error) {
	var member workspaces.Membership
	member.ID, member.WorkspaceID, member.Role, member.Status = uuid.New(), workspaceID, input.Role, workspaces.MembershipActive
	err := s.pool.QueryRow(ctx, `
INSERT INTO workspace_memberships (id, workspace_id, user_id, role, status)
SELECT $1, $2, u.id, $3, 'active' FROM users u
WHERE lower(u.email) = lower($4) AND u.status = 'active'
RETURNING id, workspace_id, user_id, role, status, supervisor_membership_id, created_at, updated_at`,
		member.ID, workspaceID, input.Role, strings.TrimSpace(input.Email)).Scan(
		&member.ID, &member.WorkspaceID, &member.UserID, &member.Role, &member.Status,
		&member.SupervisorMembershipID, &member.CreatedAt, &member.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return workspaces.Membership{}, workspaces.ErrMembershipNotFound
	}
	if err != nil {
		return workspaces.Membership{}, fmt.Errorf("create membership: %w", err)
	}
	member.Email = strings.TrimSpace(input.Email)
	return member, nil
}

func (s *PostgresStore) Update(ctx context.Context, workspaceID, membershipID uuid.UUID, input UpdateInput) (workspaces.Membership, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return workspaces.Membership{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	current, err := scanMembership(tx.QueryRow(ctx, `
SELECT m.id, m.workspace_id, m.user_id, u.email, m.role, m.status,
       m.supervisor_membership_id, m.created_at, m.updated_at
FROM workspace_memberships m JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = $1 AND m.id = $2 FOR UPDATE`, workspaceID, membershipID))
	if errors.Is(err, pgx.ErrNoRows) {
		return workspaces.Membership{}, workspaces.ErrMembershipNotFound
	}
	if err != nil {
		return workspaces.Membership{}, err
	}
	if input.Role != nil {
		current.Role = *input.Role
	}
	if input.Status != nil {
		current.Status = *input.Status
	}
	if input.ClearSupervisor {
		current.SupervisorMembershipID = nil
	} else if input.SupervisorMembershipID != nil {
		var valid bool
		err := tx.QueryRow(ctx, `SELECT EXISTS (
SELECT 1 FROM workspace_memberships
WHERE workspace_id = $1 AND id = $2 AND role = 'supervisor' AND status = 'active')`, workspaceID, *input.SupervisorMembershipID).Scan(&valid)
		if err != nil || !valid {
			return workspaces.Membership{}, workspaces.ErrInvalidSupervisor
		}
		current.SupervisorMembershipID = input.SupervisorMembershipID
	}
	if current.Role != workspaces.RoleManager {
		current.SupervisorMembershipID = nil
	}
	err = tx.QueryRow(ctx, `
UPDATE workspace_memberships
SET role = $3, status = $4, supervisor_membership_id = $5, updated_at = NOW()
WHERE workspace_id = $1 AND id = $2
RETURNING created_at, updated_at`, workspaceID, membershipID, current.Role, current.Status, current.SupervisorMembershipID).Scan(&current.CreatedAt, &current.UpdatedAt)
	if err != nil {
		return workspaces.Membership{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return workspaces.Membership{}, err
	}
	return current, nil
}

func (s *PostgresStore) Delete(ctx context.Context, workspaceID, membershipID uuid.UUID) error {
	result, err := s.pool.Exec(ctx, `DELETE FROM workspace_memberships WHERE workspace_id = $1 AND id = $2`, workspaceID, membershipID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return workspaces.ErrMembershipNotFound
	}
	return nil
}

type rowScanner interface{ Scan(...any) error }

func scanMembership(row rowScanner) (workspaces.Membership, error) {
	var member workspaces.Membership
	err := row.Scan(&member.ID, &member.WorkspaceID, &member.UserID, &member.Email, &member.Role, &member.Status,
		&member.SupervisorMembershipID, &member.CreatedAt, &member.UpdatedAt)
	return member, err
}

var _ Store = (*PostgresStore)(nil)
