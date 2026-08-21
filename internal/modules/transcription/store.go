package transcription

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TranscriptStore persists at most one transcript per call and exposes whether
// an earlier attempt already reached this durable pipeline checkpoint.
type TranscriptStore interface {
	Get(ctx context.Context, callID uuid.UUID) (Transcript, bool, error)
	Upsert(ctx context.Context, callID uuid.UUID, transcript Transcript) error
}

// PostgresStore persists transcripts through PostgreSQL's unique call_id key.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a transcript store backed by a PostgreSQL pool.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// Get retrieves a persisted transcript. The bool is false when no checkpoint exists.
func (s *PostgresStore) Get(ctx context.Context, callID uuid.UUID) (Transcript, bool, error) {
	return s.get(ctx, uuid.Nil, callID)
}

func (s *PostgresStore) GetInWorkspace(ctx context.Context, workspaceID, callID uuid.UUID) (Transcript, bool, error) {
	return s.get(ctx, workspaceID, callID)
}

func (s *PostgresStore) get(ctx context.Context, workspaceID, callID uuid.UUID) (Transcript, bool, error) {
	var transcript Transcript
	var segmentsJSON []byte
	query := `
SELECT full_text, segments
FROM call_transcripts t`
	args := []any{callID}
	if workspaceID == uuid.Nil {
		query += " WHERE t.call_id = $1"
	} else {
		query += " JOIN calls c ON c.id = t.call_id WHERE t.call_id = $1 AND c.workspace_id = $2"
		args = append(args, workspaceID)
	}
	err := s.pool.QueryRow(ctx, query, args...).Scan(&transcript.Text, &segmentsJSON)
	if errorsIsNoRows(err) {
		return Transcript{}, false, nil
	}
	if err != nil {
		return Transcript{}, false, fmt.Errorf("load transcript: %w", err)
	}
	if err := json.Unmarshal(segmentsJSON, &transcript.Segments); err != nil {
		return Transcript{}, false, fmt.Errorf("decode transcript segments: %w", err)
	}
	return transcript, true, nil
}

// Upsert records the durable transcription checkpoint before analysis starts.
func (s *PostgresStore) Upsert(ctx context.Context, callID uuid.UUID, transcript Transcript) error {
	segmentsJSON, err := json.Marshal(transcript.Segments)
	if err != nil {
		return fmt.Errorf("encode transcript segments: %w", err)
	}
	_, err = s.pool.Exec(ctx, `
INSERT INTO call_transcripts (call_id, full_text, segments)
VALUES ($1, $2, $3)
ON CONFLICT (call_id) DO UPDATE
SET full_text = EXCLUDED.full_text,
    segments = EXCLUDED.segments,
    updated_at = NOW()`, callID, transcript.Text, segmentsJSON)
	if err != nil {
		return fmt.Errorf("upsert transcript: %w", err)
	}
	return nil
}

func errorsIsNoRows(err error) bool {
	return err == pgx.ErrNoRows
}

var _ TranscriptStore = (*PostgresStore)(nil)
