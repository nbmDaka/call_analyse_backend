package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"call_analyse_backend/internal/modules/scoring"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AnalysisStore owns the durable analysis/score completion checkpoint.
type AnalysisStore interface {
	Exists(ctx context.Context, callID uuid.UUID) (bool, error)
	UpsertWithScore(ctx context.Context, callID uuid.UUID, result Analysis, score scoring.Score) error
}

// PostgresStore persists analyses and their backend-owned scores.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates an analysis store backed by a PostgreSQL pool.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// Exists reports whether a completed analysis/score result has already been persisted.
func (s *PostgresStore) Exists(ctx context.Context, callID uuid.UUID) (bool, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM call_analyses WHERE call_id = $1)`, callID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check analysis: %w", err)
	}
	return exists, nil
}

// Get loads the persisted analysis and score read model for API detail views.
func (s *PostgresStore) Get(ctx context.Context, callID uuid.UUID) (Analysis, scoring.Score, bool, error) {
	return s.get(ctx, uuid.Nil, callID)
}

func (s *PostgresStore) GetInWorkspace(ctx context.Context, workspaceID, callID uuid.UUID) (Analysis, scoring.Score, bool, error) {
	return s.get(ctx, workspaceID, callID)
}

func (s *PostgresStore) get(ctx context.Context, workspaceID, callID uuid.UUID) (Analysis, scoring.Score, bool, error) {
	var result Analysis
	var needsJSON, objectionsJSON, mistakesJSON, strengthsJSON, criteriaJSON []byte
	var speechJSON, violationsJSON, coachingJSON, roleJSON []byte
	var rawJSON []byte
	analysisQuery := `
SELECT summary, needs, objections, refusal_reason, mistakes, strengths, next_action, criterion_results,
       COALESCE(speech_analytics, '{}'::jsonb),
       COALESCE(violations, '[]'::jsonb),
       COALESCE(actionable_coaching, '[]'::jsonb),
       COALESCE(role_mapping, '{}'::jsonb),
       COALESCE(raw_response, 'null'::jsonb)
FROM call_analyses a`
	args := []any{callID}
	if workspaceID == uuid.Nil {
		analysisQuery += " WHERE a.call_id = $1"
	} else {
		analysisQuery += " JOIN calls c ON c.id = a.call_id WHERE a.call_id = $1 AND c.workspace_id = $2"
		args = append(args, workspaceID)
	}
	err := s.pool.QueryRow(ctx, analysisQuery, args...).Scan(
		&result.Summary,
		&needsJSON,
		&objectionsJSON,
		&result.RefusalReason,
		&mistakesJSON,
		&strengthsJSON,
		&result.NextAction,
		&criteriaJSON,
		&speechJSON,
		&violationsJSON,
		&coachingJSON,
		&roleJSON,
		&rawJSON,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Analysis{}, scoring.Score{}, false, nil
	}
	if err != nil {
		return Analysis{}, scoring.Score{}, false, fmt.Errorf("load analysis: %w", err)
	}
	for _, item := range []struct {
		data   []byte
		target any
	}{
		{needsJSON, &result.Needs},
		{objectionsJSON, &result.Objections},
		{mistakesJSON, &result.Mistakes},
		{strengthsJSON, &result.Strengths},
		{violationsJSON, &result.Violations},
		{coachingJSON, &result.ActionableCoaching},
	} {
		if len(item.data) > 0 {
			if err := json.Unmarshal(item.data, item.target); err != nil {
				return Analysis{}, scoring.Score{}, false, fmt.Errorf("decode analysis: %w", err)
			}
		}
	}
	if len(criteriaJSON) > 0 {
		if err := json.Unmarshal(criteriaJSON, &result.CriterionResults); err != nil {
			return Analysis{}, scoring.Score{}, false, fmt.Errorf("decode criterion results: %w", err)
		}
	}
	if len(speechJSON) > 0 && string(speechJSON) != "{}" && string(speechJSON) != "null" {
		var speech SpeechAnalytics
		if err := json.Unmarshal(speechJSON, &speech); err == nil {
			result.SpeechAnalytics = &speech
		}
	}
	if len(roleJSON) > 0 && string(roleJSON) != "{}" && string(roleJSON) != "null" {
		var role RoleMapping
		if err := json.Unmarshal(roleJSON, &role); err == nil {
			result.RoleMapping = &role
		}
	}
	if result.Violations == nil {
		result.Violations = []Violation{}
	}
	if result.ActionableCoaching == nil {
		result.ActionableCoaching = []string{}
	}
	result.RawJSON = rawJSON

	var values [9]int
	scoreQuery := `
SELECT greeting_score, rapport_score, needs_discovery_score, presentation_score,
       objection_handling_score, next_action_score, communication_score, closing_score, total_score
FROM call_scores s`
	if workspaceID == uuid.Nil {
		scoreQuery += " WHERE s.call_id = $1"
	} else {
		scoreQuery += " JOIN calls c ON c.id = s.call_id WHERE s.call_id = $1 AND c.workspace_id = $2"
	}
	err = s.pool.QueryRow(ctx, scoreQuery, args...).Scan(&values[0], &values[1], &values[2], &values[3], &values[4], &values[5], &values[6], &values[7], &values[8])
	if errors.Is(err, pgx.ErrNoRows) {
		return result, scoring.Score{}, true, nil
	}
	if err != nil {
		return Analysis{}, scoring.Score{}, false, fmt.Errorf("load score: %w", err)
	}
	keys := []string{scoring.CriterionGreeting, scoring.CriterionRapport, scoring.CriterionNeedsDiscovery, scoring.CriterionPresentation, scoring.CriterionObjectionHandling, scoring.CriterionNextAction, scoring.CriterionCommunication, scoring.CriterionClosing}
	score := scoring.Score{Criteria: make(map[string]scoring.CriterionScore, len(keys)), Total: values[8]}
	for i, key := range keys {
		score.Criteria[key] = scoring.CriterionScore{Score: values[i]}
	}
	return result, score, true, nil
}

// UpsertWithScore atomically records both result rows so a retry never observes
// an analysis without its calculated backend score.
func (s *PostgresStore) UpsertWithScore(ctx context.Context, callID uuid.UUID, result Analysis, score scoring.Score) error {
	needsJSON, err := json.Marshal(result.Needs)
	if err != nil {
		return fmt.Errorf("encode analysis needs: %w", err)
	}
	objectionsJSON, err := json.Marshal(result.Objections)
	if err != nil {
		return fmt.Errorf("encode analysis objections: %w", err)
	}
	mistakesJSON, err := json.Marshal(result.Mistakes)
	if err != nil {
		return fmt.Errorf("encode analysis mistakes: %w", err)
	}
	strengthsJSON, err := json.Marshal(result.Strengths)
	if err != nil {
		return fmt.Errorf("encode analysis strengths: %w", err)
	}
	criteriaJSON, err := json.Marshal(result.CriterionResults)
	if err != nil {
		return fmt.Errorf("encode criterion results: %w", err)
	}
	speechJSON, err := json.Marshal(result.SpeechAnalytics)
	if err != nil || result.SpeechAnalytics == nil {
		speechJSON = []byte("{}")
	}
	violationsJSON, err := json.Marshal(result.Violations)
	if err != nil || result.Violations == nil {
		violationsJSON = []byte("[]")
	}
	coachingJSON, err := json.Marshal(result.ActionableCoaching)
	if err != nil || result.ActionableCoaching == nil {
		coachingJSON = []byte("[]")
	}
	roleJSON, err := json.Marshal(result.RoleMapping)
	if err != nil || result.RoleMapping == nil {
		roleJSON = []byte("{}")
	}

	transaction, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin result transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	if _, err := transaction.Exec(ctx, `
INSERT INTO call_analyses (
    call_id, summary, needs, objections, refusal_reason, mistakes, strengths, next_action, criterion_results,
    speech_analytics, violations, actionable_coaching, role_mapping, raw_response
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NULLIF($14::jsonb, 'null'::jsonb))
ON CONFLICT (call_id) DO UPDATE
SET summary = EXCLUDED.summary,
    needs = EXCLUDED.needs,
    objections = EXCLUDED.objections,
    refusal_reason = EXCLUDED.refusal_reason,
    mistakes = EXCLUDED.mistakes,
    strengths = EXCLUDED.strengths,
    next_action = EXCLUDED.next_action,
    criterion_results = EXCLUDED.criterion_results,
    speech_analytics = EXCLUDED.speech_analytics,
    violations = EXCLUDED.violations,
    actionable_coaching = EXCLUDED.actionable_coaching,
    role_mapping = EXCLUDED.role_mapping,
    raw_response = EXCLUDED.raw_response,
    updated_at = NOW()`,
		callID,
		result.Summary,
		needsJSON,
		objectionsJSON,
		result.RefusalReason,
		mistakesJSON,
		strengthsJSON,
		result.NextAction,
		criteriaJSON,
		speechJSON,
		violationsJSON,
		coachingJSON,
		roleJSON,
		rawJSONOrNull(result.RawJSON),
	); err != nil {
		return fmt.Errorf("upsert analysis: %w", err)
	}

	if _, err := transaction.Exec(ctx, `
INSERT INTO call_scores (
    call_id, greeting_score, rapport_score, needs_discovery_score, presentation_score,
    objection_handling_score, next_action_score, communication_score, closing_score, total_score
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (call_id) DO UPDATE
SET greeting_score = EXCLUDED.greeting_score,
    rapport_score = EXCLUDED.rapport_score,
    needs_discovery_score = EXCLUDED.needs_discovery_score,
    presentation_score = EXCLUDED.presentation_score,
    objection_handling_score = EXCLUDED.objection_handling_score,
    next_action_score = EXCLUDED.next_action_score,
    communication_score = EXCLUDED.communication_score,
    closing_score = EXCLUDED.closing_score,
    total_score = EXCLUDED.total_score,
    updated_at = NOW()`,
		callID,
		scoreFor(score, scoring.CriterionGreeting),
		scoreFor(score, scoring.CriterionRapport),
		scoreFor(score, scoring.CriterionNeedsDiscovery),
		scoreFor(score, scoring.CriterionPresentation),
		scoreFor(score, scoring.CriterionObjectionHandling),
		scoreFor(score, scoring.CriterionNextAction),
		scoreFor(score, scoring.CriterionCommunication),
		scoreFor(score, scoring.CriterionClosing),
		score.Total,
	); err != nil {
		return fmt.Errorf("upsert score: %w", err)
	}

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit result transaction: %w", err)
	}
	return nil
}

func rawJSONOrNull(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return []byte("null")
	}
	return raw
}

func scoreFor(score scoring.Score, criterion string) int {
	return score.Criteria[criterion].Score
}

var _ AnalysisStore = (*PostgresStore)(nil)
