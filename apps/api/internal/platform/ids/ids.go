// Package ids generates the primary keys of every table: UUIDv7, generated in Go (never by the
// database), so ids sort by creation time and can be known before the INSERT.
package ids

import "github.com/google/uuid"

// New returns a new UUIDv7. uuid.NewV7 only fails when the OS random source fails, which
// crypto/rand already treats as fatal, so Must cannot hide a recoverable error here.
func New() uuid.UUID {
	return uuid.Must(uuid.NewV7())
}

// Parse parses a textual UUID (any version).
func Parse(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
