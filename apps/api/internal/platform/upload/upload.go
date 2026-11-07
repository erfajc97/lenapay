// Package upload holds the rules every file upload shares (A13): the accepted types decided by
// the file's magic bytes (never by the client's declared type or name), the size cap, the
// cleaned display name and the content hash stored next to the object.
package upload

import (
	"bytes"
	"crypto/sha256"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxBytes is the contract's 10 MB per uploaded file.
const MaxBytes = 10 << 20

// Accepted upload types (A13: never SVG or HTML).
const (
	TypePDF  = "application/pdf"
	TypeJPEG = "image/jpeg"
	TypePNG  = "image/png"
)

// maxFileNameRunes is the file_name CHECK of every table that stores uploads.
const maxFileNameRunes = 200

// Sniff returns the real type of content from its magic bytes, or "" when it is not an
// accepted type (whatever the client declared).
func Sniff(content []byte) string {
	switch {
	case bytes.HasPrefix(content, []byte("%PDF-")):
		return TypePDF
	case bytes.HasPrefix(content, []byte{0xFF, 0xD8, 0xFF}):
		return TypeJPEG
	case bytes.HasPrefix(content, []byte("\x89PNG\r\n\x1a\n")):
		return TypePNG
	}
	return ""
}

// CleanFileName keeps the base name only, drops control characters and caps it to the column.
func CleanFileName(name string) string {
	// path, not filepath: the base name must be the same on every OS the API runs on.
	name = path.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == utf8.RuneError {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == "/" {
		return "document"
	}
	if utf8.RuneCountInString(name) > maxFileNameRunes {
		name = string([]rune(name)[:maxFileNameRunes])
	}
	return name
}

// SHA256 is the content hash stored with the object.
func SHA256(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}
