package calls

import (
	"fmt"
	"mime"
	"path/filepath"
	"strings"
)

var allowedAudioTypes = map[string]map[string]struct{}{
	".mp3": {
		"audio/mpeg": {},
	},
	".wav": {
		"audio/wav":   {},
		"audio/x-wav": {},
	},
	".m4a": {
		"audio/mp4":   {},
		"audio/x-m4a": {},
	},
	".ogg": {
		"audio/ogg":       {},
		"application/ogg": {},
		"audio/opus":      {},
		"audio/x-ogg":     {},
		"audio/vorbis":    {},
	},
	".oga": {
		"audio/ogg":       {},
		"application/ogg": {},
	},
	".opus": {
		"audio/ogg":  {},
		"audio/opus": {},
	},
}

// ValidateUpload verifies that untrusted upload metadata describes a supported,
// bounded audio file. The caller must still enforce the byte limit while reading.
func ValidateUpload(filename, contentType string, size int64, maxBytes int64) error {
	if filename == "" || strings.ContainsAny(filename, "/\\") {
		return fmt.Errorf("invalid upload filename")
	}
	if size < 0 || maxBytes <= 0 || size > maxBytes {
		return fmt.Errorf("upload size must be between 0 and %d bytes", maxBytes)
	}

	extension := strings.ToLower(filepath.Ext(filename))
	allowedMIMEs, ok := allowedAudioTypes[extension]
	if !ok {
		return fmt.Errorf("unsupported audio extension")
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return fmt.Errorf("invalid upload content type")
	}
	if _, ok := allowedMIMEs[strings.ToLower(mediaType)]; !ok {
		return fmt.Errorf("content type does not match file extension")
	}
	return nil
}
