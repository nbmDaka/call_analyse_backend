// Package dashboard provides scoped aggregate read models for the API dashboard.
package dashboard

import (
	"context"
	"fmt"

	"call_analyse_backend/internal/modules/calls"
	"call_analyse_backend/internal/modules/workspaces"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Summary is the bounded dashboard aggregate available to every authenticated role.
type Summary struct {
	TotalCalls     int      `json:"total_calls"`
	CompletedCalls int      `json:"completed_calls"`
	FailedCalls    int      `json:"failed_calls"`
	AverageScore   *float64 `json:"average_score"`
}

// Store executes aggregate queries after applying the caller's visibility scope.
type Store interface {
	Summary(ctx context.Context, actor calls.Actor) (Summary, error)
}

// PostgresStore reads dashboard aggregates from PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a dashboard store backed by a PostgreSQL pool.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// Summary returns counts and the score average only for calls visible to actor.
func (s *PostgresStore) Summary(ctx context.Context, actor calls.Actor) (Summary, error) {
	query, args, err := summaryQuery(actor)
	if err != nil {
		return Summary{}, err
	}
	var summary Summary
	if err := s.pool.QueryRow(ctx, query, args...).Scan(
		&summary.TotalCalls,
		&summary.CompletedCalls,
		&summary.FailedCalls,
		&summary.AverageScore,
	); err != nil {
		return Summary{}, fmt.Errorf("query dashboard summary: %w", err)
	}
	return summary, nil
}

func summaryQuery(actor calls.Actor) (string, []any, error) {
	where, args, err := dashboardScope(actor)
	if err != nil {
		return "", nil, err
	}
	return `
SELECT
    COUNT(c.id)::integer,
    COUNT(c.id) FILTER (WHERE c.status = 'completed')::integer,
    COUNT(c.id) FILTER (WHERE c.status = 'failed')::integer,
    AVG(s.total_score)::double precision
FROM calls c
LEFT JOIN call_scores s ON s.call_id = c.id
WHERE ` + where, args, nil
}

func dashboardScope(actor calls.Actor) (string, []any, error) {
	if actor.WorkspaceID == uuid.Nil || actor.UserID == uuid.Nil || actor.MembershipID == uuid.Nil {
		return "", nil, calls.ErrInvalidActor
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
		return "", nil, calls.ErrInvalidActor
	}
}

var _ Store = (*PostgresStore)(nil)
