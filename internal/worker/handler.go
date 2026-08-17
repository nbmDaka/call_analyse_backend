package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"call_analyse_backend/internal/queue"

	"github.com/hibiken/asynq"
)

// Handler adapts Processor to Asynq without swallowing processing errors.
type Handler struct {
	processor *Processor
}

// NewHandler constructs an Asynq handler for process_call tasks.
func NewHandler(processor *Processor) *Handler {
	return &Handler{processor: processor}
}

// ProcessTask validates the stable payload then returns the processor error so
// Asynq can apply the task's configured retry policy.
func (h *Handler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	if h.processor == nil {
		return fmt.Errorf("worker processor is required")
	}
	if task.Type() != queue.TypeProcessCall {
		return fmt.Errorf("unsupported task type %q", task.Type())
	}
	var payload queue.ProcessCallPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("decode process call task: %w", err)
	}
	return h.processor.Process(ctx, payload.CallID)
}
