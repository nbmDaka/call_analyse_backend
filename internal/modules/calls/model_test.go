package calls

import "testing"

func TestCanTransitionAllowsProcessingPath(t *testing.T) {
	path := []Status{
		StatusUploaded,
		StatusQueued,
		StatusTranscribing,
		StatusTranscribed,
		StatusAnalyzing,
		StatusCompleted,
	}

	for i := 0; i < len(path)-1; i++ {
		if !CanTransition(path[i], path[i+1]) {
			t.Errorf("CanTransition(%q, %q) = false, want true", path[i], path[i+1])
		}
	}
}

func TestCanTransitionAllowsFailureFromProcessingStates(t *testing.T) {
	for _, from := range []Status{
		StatusQueued,
		StatusTranscribing,
		StatusTranscribed,
		StatusAnalyzing,
	} {
		if !CanTransition(from, StatusFailed) {
			t.Errorf("CanTransition(%q, %q) = false, want true", from, StatusFailed)
		}
	}
}

func TestValidateTransitionRejectsCompletedToTranscribing(t *testing.T) {
	if err := ValidateTransition(StatusCompleted, StatusTranscribing); err == nil {
		t.Fatal("ValidateTransition(completed, transcribing) error = nil, want error")
	}
}
