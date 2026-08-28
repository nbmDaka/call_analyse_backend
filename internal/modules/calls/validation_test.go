package calls

import "testing"

func TestValidateUploadAcceptsSupportedAudioTypesCaseInsensitively(t *testing.T) {
	maxBytes := int64(100)
	for _, upload := range []struct {
		filename    string
		contentType string
	}{
		{"recording.MP3", "audio/mpeg"},
		{"recording.WaV", "audio/wav"},
		{"recording.m4A", "audio/mp4"},
		{"recording.OGG", "audio/ogg"},
		{"recording.opus", "audio/opus"},
	} {
		if err := ValidateUpload(upload.filename, upload.contentType, maxBytes, maxBytes); err != nil {
			t.Errorf("ValidateUpload(%q, %q) error = %v, want nil", upload.filename, upload.contentType, err)
		}
	}
}

func TestValidateUploadRejectsUnsafeOrMismatchedMetadata(t *testing.T) {
	maxBytes := int64(100)
	for _, upload := range []struct {
		name        string
		filename    string
		contentType string
		size        int64
	}{
		{"path separator", "../recording.mp3", "audio/mpeg", 1},
		{"unsupported extension", "recording.flac", "audio/flac", 1},
		{"mime mismatch", "recording.mp3", "audio/wav", 1},
		{"unsupported mime", "recording.wav", "application/octet-stream", 1},
		{"over limit", "recording.m4a", "audio/mp4", maxBytes + 1},
	} {
		t.Run(upload.name, func(t *testing.T) {
			if err := ValidateUpload(upload.filename, upload.contentType, upload.size, maxBytes); err == nil {
				t.Fatal("ValidateUpload() error = nil, want validation error")
			}
		})
	}
}
