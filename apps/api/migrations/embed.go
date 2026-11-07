// Package migrations embeds the hand-written goose SQL migrations (A11) so the API binary,
// the test helpers and sqlc all read the same files.
package migrations

import "embed"

// FS holds every numbered migration; goose applies them in order at startup.
//
//go:embed *.sql
var FS embed.FS
