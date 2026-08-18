// Package transcription defines transcript data shared by providers and workers.
package transcription

// Speaker is the constrained label for a transcript segment participant.
type Speaker string

const (
	SpeakerManager Speaker = "manager"
	SpeakerClient  Speaker = "client"
)

// AudioInput is the audio material supplied to a transcription provider.
type AudioInput struct {
	Filename string
	MIMEType string
	Data     []byte
}

// Segment is a speaker-attributed part of a transcript. Timestamps remain nil
// when the provider cannot supply them.
type Segment struct {
	Speaker Speaker  `json:"speaker"`
	Text    string   `json:"text"`
	Start   *float64 `json:"start_seconds,omitempty"`
	End     *float64 `json:"end_seconds,omitempty"`
}

// TranscriptResult is the complete text and optional segments returned by a provider.
type TranscriptResult struct {
	Text     string    `json:"text"`
	Segments []Segment `json:"segments"`
}

// Transcript is retained as the worker-facing name for a persisted transcript.
type Transcript = TranscriptResult
