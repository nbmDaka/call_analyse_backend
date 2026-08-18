// Package queue defines stable Asynq task contracts.
package queue

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const (
	// TypeProcessCall is the stable Asynq task type for a call-processing job.
	TypeProcessCall     = "process_call"
	ProcessCallQueue    = "calls"
	ProcessCallMaxRetry = 5
)

var processCallUniqueFor = 24 * time.Hour

// ProcessCallPayload is intentionally small and stable so the task uniqueness
// key is deterministically derived from the call ID payload.
type ProcessCallPayload struct {
	CallID string `json:"call_id"`
}

// NewProcessCallTask creates a deduplicated, bounded-retry call-processing task.
func NewProcessCallTask(callID string) (*asynq.Task, error) {
	parsedID, err := uuid.Parse(callID)
	if err != nil {
		return nil, fmt.Errorf("parse call ID: %w", err)
	}
	payload, err := json.Marshal(ProcessCallPayload{CallID: parsedID.String()})
	if err != nil {
		return nil, fmt.Errorf("marshal process call task payload: %w", err)
	}
	return asynq.NewTask(TypeProcessCall, payload,
		asynq.Queue(ProcessCallQueue),
		asynq.MaxRetry(ProcessCallMaxRetry),
		asynq.Unique(processCallUniqueFor),
	), nil
}
