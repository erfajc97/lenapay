// Package page implements the keyset cursor pagination shared by every list ({items, nextCursor}).
// A cursor is the (created_at, id) of the last row of the previous page, opaque to clients.
package page

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
)

// Limits of a page (A3: a floor and a ceiling).
const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// Request is a validated page request.
type Request struct {
	// After is the position of the last row already seen; nil for the first page.
	After *Position
	Limit int
}

// Position is a keyset position: rows strictly before it in (created_at DESC, id DESC) order.
type Position struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// Page is one page of results.
type Page[T any] struct {
	Items      []T
	NextCursor *string
}

// ParseRequest validates the cursor and limit query parameters.
func ParseRequest(cursor *string, limit *int) (Request, error) {
	req := Request{Limit: DefaultLimit}
	if limit != nil {
		if *limit < 1 || *limit > MaxLimit {
			return Request{}, apperr.New(apperr.CodeValidation, map[string]any{"param": "limit"})
		}
		req.Limit = *limit
	}
	if cursor != nil && *cursor != "" {
		pos, err := Decode(*cursor)
		if err != nil {
			return Request{}, apperr.New(apperr.CodeValidation, map[string]any{"param": "cursor"})
		}
		req.After = &pos
	}
	return req, nil
}

// Encode turns a position into an opaque cursor.
func Encode(p Position) string {
	raw := strconv.FormatInt(p.CreatedAt.UnixMicro(), 10) + "." + p.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// Decode parses a cursor produced by Encode.
func Decode(cursor string) (Position, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return Position{}, fmt.Errorf("cursor encoding: %w", err)
	}
	micros, id, ok := strings.Cut(string(raw), ".")
	if !ok {
		return Position{}, fmt.Errorf("cursor shape")
	}
	us, err := strconv.ParseInt(micros, 10, 64)
	if err != nil {
		return Position{}, fmt.Errorf("cursor time: %w", err)
	}
	uid, err := uuid.Parse(id)
	if err != nil {
		return Position{}, fmt.Errorf("cursor id: %w", err)
	}
	return Position{CreatedAt: time.UnixMicro(us).UTC(), ID: uid}, nil
}

// Build cuts rows (fetched with Limit+1) into a page: the extra row only signals that a next
// page exists, and the cursor points at the last row returned.
func Build[T any](rows []T, limit int, position func(T) Position) Page[T] {
	if len(rows) <= limit {
		return Page[T]{Items: rows}
	}
	items := rows[:limit]
	next := Encode(position(items[len(items)-1]))
	return Page[T]{Items: items, NextCursor: &next}
}

// FetchLimit is the LIMIT to send to the query: one more row than the page.
func (r Request) FetchLimit() int32 {
	return int32(r.Limit + 1) //nolint:gosec // ParseRequest caps Limit at MaxLimit (100)
}
