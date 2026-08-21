package golden_standards

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrGoldenStandardNotFound = errors.New("golden standard not found")
)

// Store defines persistence operations for workspace golden standards.
type Store interface {
	List(ctx context.Context, workspaceID uuid.UUID, category string) ([]GoldenStandard, error)
	GetByID(ctx context.Context, workspaceID, id uuid.UUID) (GoldenStandard, error)
	Create(ctx context.Context, standard GoldenStandard) (GoldenStandard, error)
	Delete(ctx context.Context, workspaceID, id uuid.UUID) error
}

// PostgresStore implements Store using PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore returns a new PostgreSQL golden standards store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// List returns golden standards for a workspace, optionally filtered by category.
func (s *PostgresStore) List(ctx context.Context, workspaceID uuid.UUID, category string) ([]GoldenStandard, error) {
	query := `
SELECT id, workspace_id, call_id, category, title, transcript_snippet,
       audio_start_seconds, audio_end_seconds, why_golden, created_at, updated_at
FROM golden_standards
WHERE workspace_id = $1`
	args := []any{workspaceID}
	if category != "" {
		query += " AND category = $2"
		args = append(args, category)
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list golden standards: %w", err)
	}
	defer rows.Close()

	var list []GoldenStandard
	for rows.Next() {
		var g GoldenStandard
		if err := rows.Scan(
			&g.ID, &g.WorkspaceID, &g.CallID, &g.Category, &g.Title, &g.TranscriptSnippet,
			&g.AudioStartSeconds, &g.AudioEndSeconds, &g.WhyGolden, &g.CreatedAt, &g.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan golden standard: %w", err)
		}
		list = append(list, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate golden standards: %w", err)
	}
	if list == nil {
		list = []GoldenStandard{}
	}
	return list, nil
}

// GetByID returns one golden standard.
func (s *PostgresStore) GetByID(ctx context.Context, workspaceID, id uuid.UUID) (GoldenStandard, error) {
	var g GoldenStandard
	err := s.pool.QueryRow(ctx, `
SELECT id, workspace_id, call_id, category, title, transcript_snippet,
       audio_start_seconds, audio_end_seconds, why_golden, created_at, updated_at
FROM golden_standards
WHERE workspace_id = $1 AND id = $2`, workspaceID, id).Scan(
		&g.ID, &g.WorkspaceID, &g.CallID, &g.Category, &g.Title, &g.TranscriptSnippet,
		&g.AudioStartSeconds, &g.AudioEndSeconds, &g.WhyGolden, &g.CreatedAt, &g.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return GoldenStandard{}, ErrGoldenStandardNotFound
	}
	if err != nil {
		return GoldenStandard{}, fmt.Errorf("get golden standard by ID: %w", err)
	}
	return g, nil
}

// Create inserts a new golden standard.
func (s *PostgresStore) Create(ctx context.Context, standard GoldenStandard) (GoldenStandard, error) {
	var created GoldenStandard
	err := s.pool.QueryRow(ctx, `
INSERT INTO golden_standards (
    workspace_id, call_id, category, title, transcript_snippet,
    audio_start_seconds, audio_end_seconds, why_golden
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, workspace_id, call_id, category, title, transcript_snippet,
          audio_start_seconds, audio_end_seconds, why_golden, created_at, updated_at`,
		standard.WorkspaceID, standard.CallID, standard.Category, standard.Title, standard.TranscriptSnippet,
		standard.AudioStartSeconds, standard.AudioEndSeconds, standard.WhyGolden,
	).Scan(
		&created.ID, &created.WorkspaceID, &created.CallID, &created.Category, &created.Title, &created.TranscriptSnippet,
		&created.AudioStartSeconds, &created.AudioEndSeconds, &created.WhyGolden, &created.CreatedAt, &created.UpdatedAt,
	)
	if err != nil {
		return GoldenStandard{}, fmt.Errorf("insert golden standard: %w", err)
	}
	return created, nil
}

// Delete removes a golden standard.
func (s *PostgresStore) Delete(ctx context.Context, workspaceID, id uuid.UUID) error {
	cmd, err := s.pool.Exec(ctx, `DELETE FROM golden_standards WHERE workspace_id = $1 AND id = $2`, workspaceID, id)
	if err != nil {
		return fmt.Errorf("delete golden standard: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrGoldenStandardNotFound
	}
	return nil
}

var _ Store = (*PostgresStore)(nil)
