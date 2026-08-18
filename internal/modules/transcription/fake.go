package transcription

import "context"

// FakeProvider supplies deterministic transcript data for development and tests.
type FakeProvider struct{}

// Transcribe returns fixed, speaker-labelled content and deliberately omits
// timestamps because the fake provider has no timing information.
func (FakeProvider) Transcribe(_ context.Context, _ AudioInput) (TranscriptResult, error) {
	return TranscriptResult{
		Text: "manager: Hello, thank you for taking the call.\nclient: I need a proposal that fits our budget.",
		Segments: []Segment{
			{Speaker: SpeakerManager, Text: "Hello, thank you for taking the call."},
			{Speaker: SpeakerClient, Text: "I need a proposal that fits our budget."},
		},
	}, nil
}

var _ TranscriptionProvider = FakeProvider{}
