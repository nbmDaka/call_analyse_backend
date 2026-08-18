package queue

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestNewProcessCallTaskRoundTripsCallID(t *testing.T) {
	callID := uuid.MustParse("00000000-0000-0000-0000-000000000123")

	task, err := NewProcessCallTask(callID.String())
	if err != nil {
		t.Fatalf("NewProcessCallTask() error = %v", err)
	}
	if task.Type() != TypeProcessCall {
		t.Errorf("task type = %q, want %q", task.Type(), TypeProcessCall)
	}

	var payload ProcessCallPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.CallID != callID.String() {
		t.Errorf("payload call_id = %q, want %q", payload.CallID, callID)
	}
}

func TestNewProcessCallTaskRejectsInvalidCallID(t *testing.T) {
	if _, err := NewProcessCallTask("not-a-uuid"); err == nil {
		t.Fatal("NewProcessCallTask() error = nil, want invalid UUID error")
	}
}
