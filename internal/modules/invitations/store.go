package invitations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"call_analyse_backend/internal/modules/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store interface {
	Create(ctx context.Context, inv Invitation, tokenHash string) (Invitation, error)
	ListPendingByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]Invitation, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (Invitation, bool, error)
	Revoke(ctx context.Context, workspaceID, invitationID uuid.UUID) error
	AcceptForUser(ctx context.Context, tokenHash string, userID uuid.UUID) (uuid.UUID, error)
	RegisterAndAccept(ctx context.Context, tokenHash string, passwordHash string) (auth.User, uuid.UUID, error)
}

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) Create(ctx context.Context, inv Invitation, tokenHash string) (Invitation, error) {
	// 1. Check if user is already an active member of the workspace
	var isMember bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM workspace_memberships wm
			JOIN users u ON u.id = wm.user_id
			WHERE wm.workspace_id = $1 AND lower(u.email) = lower($2) AND wm.status = 'active'
		)
	`, inv.WorkspaceID, inv.Email).Scan(&isMember)
	if err != nil {
		return Invitation{}, fmt.Errorf("check existing membership: %w", err)
	}
	if isMember {
		return Invitation{}, ErrAlreadyMember
	}

	// 2. Remove any previous unaccepted invitations for this email in this workspace
	_, _ = s.pool.Exec(ctx, `
		DELETE FROM workspace_invitations
		WHERE workspace_id = $1 AND lower(email) = lower($2) AND accepted_at IS NULL
	`, inv.WorkspaceID, inv.Email)

	// 3. Insert new invitation
	if inv.ID == uuid.Nil {
		inv.ID = uuid.New()
	}
	now := time.Now().UTC()
	inv.CreatedAt = now
	inv.UpdatedAt = now

	err = s.pool.QueryRow(ctx, `
		INSERT INTO workspace_invitations (id, workspace_id, email, role, token_hash, invited_by_user_id, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, workspace_id, email, role, invited_by_user_id, expires_at, accepted_at, created_at, updated_at
	`, inv.ID, inv.WorkspaceID, inv.Email, inv.Role, tokenHash, inv.InvitedByUserID, inv.ExpiresAt, inv.CreatedAt, inv.UpdatedAt).Scan(
		&inv.ID, &inv.WorkspaceID, &inv.Email, &inv.Role, &inv.InvitedByUserID, &inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt, &inv.UpdatedAt,
	)
	if err != nil {
		return Invitation{}, fmt.Errorf("insert invitation: %w", err)
	}

	// Load workspace name
	_ = s.pool.QueryRow(ctx, `SELECT name FROM workspaces WHERE id = $1`, inv.WorkspaceID).Scan(&inv.WorkspaceName)

	return inv, nil
}

func (s *PostgresStore) ListPendingByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]Invitation, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT i.id, i.workspace_id, w.name, i.email, i.role, i.invited_by_user_id, i.expires_at, i.accepted_at, i.created_at, i.updated_at
		FROM workspace_invitations i
		JOIN workspaces w ON w.id = i.workspace_id
		WHERE i.workspace_id = $1 AND i.accepted_at IS NULL AND i.expires_at > NOW()
		ORDER BY i.created_at DESC
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list pending invitations: %w", err)
	}
	defer rows.Close()

	var result []Invitation
	for rows.Next() {
		var inv Invitation
		if err := rows.Scan(
			&inv.ID, &inv.WorkspaceID, &inv.WorkspaceName, &inv.Email, &inv.Role,
			&inv.InvitedByUserID, &inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt, &inv.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan invitation: %w", err)
		}
		result = append(result, inv)
	}
	return result, rows.Err()
}

func (s *PostgresStore) GetByTokenHash(ctx context.Context, tokenHash string) (Invitation, bool, error) {
	var inv Invitation
	var isExistingUser bool

	err := s.pool.QueryRow(ctx, `
		SELECT i.id, i.workspace_id, w.name, i.email, i.role, i.invited_by_user_id, i.expires_at, i.accepted_at, i.created_at, i.updated_at,
		       EXISTS(SELECT 1 FROM users u WHERE lower(u.email) = lower(i.email) AND u.status = 'active') AS is_existing
		FROM workspace_invitations i
		JOIN workspaces w ON w.id = i.workspace_id
		WHERE i.token_hash = $1
	`, tokenHash).Scan(
		&inv.ID, &inv.WorkspaceID, &inv.WorkspaceName, &inv.Email, &inv.Role,
		&inv.InvitedByUserID, &inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt, &inv.UpdatedAt,
		&isExistingUser,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Invitation{}, false, ErrInvitationNotFound
	}
	if err != nil {
		return Invitation{}, false, fmt.Errorf("get invitation: %w", err)
	}
	if inv.AcceptedAt != nil {
		return inv, isExistingUser, ErrInvitationAccepted
	}
	if time.Now().UTC().After(inv.ExpiresAt) {
		return inv, isExistingUser, ErrInvitationExpired
	}

	return inv, isExistingUser, nil
}

func (s *PostgresStore) Revoke(ctx context.Context, workspaceID, invitationID uuid.UUID) error {
	result, err := s.pool.Exec(ctx, `
		DELETE FROM workspace_invitations
		WHERE workspace_id = $1 AND id = $2 AND accepted_at IS NULL
	`, workspaceID, invitationID)
	if err != nil {
		return fmt.Errorf("revoke invitation: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrInvitationNotFound
	}
	return nil
}

func (s *PostgresStore) AcceptForUser(ctx context.Context, tokenHash string, userID uuid.UUID) (uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var inv Invitation
	err = tx.QueryRow(ctx, `
		SELECT id, workspace_id, email, role, expires_at, accepted_at
		FROM workspace_invitations
		WHERE token_hash = $1
		FOR UPDATE
	`, tokenHash).Scan(&inv.ID, &inv.WorkspaceID, &inv.Email, &inv.Role, &inv.ExpiresAt, &inv.AcceptedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrInvitationNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}
	if inv.AcceptedAt != nil {
		return uuid.Nil, ErrInvitationAccepted
	}
	if time.Now().UTC().After(inv.ExpiresAt) {
		return uuid.Nil, ErrInvitationExpired
	}

	// Insert or update membership
	_, err = tx.Exec(ctx, `
		INSERT INTO workspace_memberships (workspace_id, user_id, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', NOW(), NOW())
		ON CONFLICT (workspace_id, user_id)
		DO UPDATE SET role = EXCLUDED.role, status = 'active', updated_at = NOW()
	`, inv.WorkspaceID, userID, inv.Role)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create workspace membership: %w", err)
	}

	// Mark accepted
	_, err = tx.Exec(ctx, `
		UPDATE workspace_invitations
		SET accepted_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, inv.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("mark invitation accepted: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return inv.WorkspaceID, nil
}

func (s *PostgresStore) RegisterAndAccept(ctx context.Context, tokenHash string, passwordHash string) (auth.User, uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return auth.User{}, uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var inv Invitation
	err = tx.QueryRow(ctx, `
		SELECT id, workspace_id, email, role, expires_at, accepted_at
		FROM workspace_invitations
		WHERE token_hash = $1
		FOR UPDATE
	`, tokenHash).Scan(&inv.ID, &inv.WorkspaceID, &inv.Email, &inv.Role, &inv.ExpiresAt, &inv.AcceptedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, uuid.Nil, ErrInvitationNotFound
	}
	if err != nil {
		return auth.User{}, uuid.Nil, err
	}
	if inv.AcceptedAt != nil {
		return auth.User{}, uuid.Nil, ErrInvitationAccepted
	}
	if time.Now().UTC().After(inv.ExpiresAt) {
		return auth.User{}, uuid.Nil, ErrInvitationExpired
	}

	var user auth.User
	userID := uuid.New()
	now := time.Now().UTC()

	err = tx.QueryRow(ctx, `
		INSERT INTO users (id, email, password_hash, platform_role, status, role, email_verified_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'user', 'active', 'manager', $4, $5, $6)
		ON CONFLICT (email)
		DO UPDATE SET password_hash = EXCLUDED.password_hash, email_verified_at = COALESCE(users.email_verified_at, EXCLUDED.email_verified_at), status = 'active', updated_at = NOW()
		RETURNING id, email, password_hash, platform_role, status, role, supervisor_id, email_verified_at, created_at, updated_at
	`, userID, inv.Email, passwordHash, now, now, now).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.PlatformRole, &user.Status, &user.Role,
		&user.SupervisorID, &user.EmailVerifiedAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return auth.User{}, uuid.Nil, fmt.Errorf("upsert invited user: %w", err)
	}

	// Insert membership into the workspace
	_, err = tx.Exec(ctx, `
		INSERT INTO workspace_memberships (workspace_id, user_id, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', NOW(), NOW())
		ON CONFLICT (workspace_id, user_id)
		DO UPDATE SET role = EXCLUDED.role, status = 'active', updated_at = NOW()
	`, inv.WorkspaceID, user.ID, inv.Role)
	if err != nil {
		return auth.User{}, uuid.Nil, fmt.Errorf("create workspace membership: %w", err)
	}

	// Mark invitation accepted
	_, err = tx.Exec(ctx, `
		UPDATE workspace_invitations
		SET accepted_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, inv.ID)
	if err != nil {
		return auth.User{}, uuid.Nil, fmt.Errorf("mark invitation accepted: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return auth.User{}, uuid.Nil, err
	}
	return user, inv.WorkspaceID, nil
}
