package transcription

import "context"

// TranscriptionProvider converts an audio input into a complete transcript.
type TranscriptionProvider interface {
	Transcribe(context.Context, AudioInput) (TranscriptResult, error)
}
