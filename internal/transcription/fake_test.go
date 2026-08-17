package transcription

import (
	"context"
	"testing"
)

func TestFakeProviderReturnsStableTranscriptWithNullableTimestamps(t *testing.T) {
	provider := FakeProvider{}
	input := AudioInput{Filename: "call.wav", MIMEType: "audio/wav", Data: []byte("audio")}

	first, err := provider.Transcribe(context.Background(), input)
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	second, err := provider.Transcribe(context.Background(), input)
	if err != nil {
		t.Fatalf("Transcribe() second error = %v", err)
	}

	if first.Text == "" {
		t.Fatal("Transcribe() text is empty")
	}
	if first.Text != second.Text {
		t.Errorf("Transcribe() text = %q then %q, want deterministic output", first.Text, second.Text)
	}
	if len(first.Segments) == 0 {
		t.Fatal("Transcribe() segments are empty")
	}
	for _, segment := range first.Segments {
		if segment.Speaker != SpeakerManager && segment.Speaker != SpeakerClient {
			t.Errorf("segment speaker = %q, want manager or client", segment.Speaker)
		}
		if segment.Text == "" {
			t.Error("segment text is empty")
		}
		if segment.Start != nil || segment.End != nil {
			t.Errorf("fake timestamps = %v, %v, want nil when unavailable", segment.Start, segment.End)
		}
	}
}
