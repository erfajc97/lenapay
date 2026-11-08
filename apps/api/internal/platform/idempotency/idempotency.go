// Package idempotency stores client idempotency keys (MO2, R-OUT-3). A key is claimed in the
// SAME transaction as the money effect and completed with the response before commit:
//
//	replay, err := store.Claim(ctx, tx, key)   // inside the money transaction
//	if replay != nil { return decode(replay.Body) }
//	... do the effect ...
//	store.Complete(ctx, tx, key, http.StatusCreated, body)
//
// A concurrent request with the same key blocks on the row until the first commits, then
// replays its response. The same key with a different request is 409 idempotency.key_reused.
package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/clock"
)

// DefaultTTL is how long a key is remembered (clients persist keys for at most 24 h).
const DefaultTTL = 24 * time.Hour

// Key identifies one client action.
type Key struct {
	// Scope names the operation and its tenant, e.g. Scope(orgID, "payouts.create").
	Scope string
	// Value is the client's Idempotency-Key header.
	Value string
	// RequestHash binds the key to the request content (HashRequest).
	RequestHash [32]byte
}

// Stored is a previously completed response, replayed as-is.
type Stored struct {
	Status int
	Body   []byte
}

// Store claims and completes keys.
type Store struct {
	clock clock.Clock
	ttl   time.Duration
}

// NewStore returns a store remembering keys for ttl.
func NewStore(c clock.Clock, ttl time.Duration) *Store {
	return &Store{clock: c, ttl: ttl}
}

// Scope builds the scope of an org operation, so two orgs never share a key space.
func Scope(orgID uuid.UUID, operation string) string {
	return "org:" + orgID.String() + ":" + operation
}

// HashRequest hashes the canonical JSON of the request (struct field order is fixed, so equal
// requests hash equally). Include everything that changes the effect, including path ids.
func HashRequest(v any) ([32]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return [32]byte{}, fmt.Errorf("hash request: %w", err)
	}
	return sha256.Sum256(b), nil
}

// Claim reserves key inside tx. It returns (nil, nil) when the caller must perform the effect,
// the stored response when the key was already completed with the same request, and
// idempotency.key_reused when it was used with a different request.
func (s *Store) Claim(ctx context.Context, tx pgx.Tx, key Key) (*Stored, error) {
	now := s.clock.Now()
	tag, err := tx.Exec(ctx, `
		INSERT INTO idempotency_keys (scope, key, request_hash, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (scope, key) DO NOTHING`,
		key.Scope, key.Value, key.RequestHash[:], now, now.Add(s.ttl))
	if err != nil {
		return nil, fmt.Errorf("claim idempotency key: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return nil, nil
	}

	var (
		hash      []byte
		status    *int
		body      []byte
		expiresAt time.Time
	)
	err = tx.QueryRow(ctx, `
		SELECT request_hash, response_status, response_body, expires_at
		FROM idempotency_keys WHERE scope = $1 AND key = $2
		FOR UPDATE`, key.Scope, key.Value).Scan(&hash, &status, &body, &expiresAt)
	if err != nil {
		return nil, fmt.Errorf("read idempotency key: %w", err)
	}

	if !expiresAt.After(now) {
		_, err := tx.Exec(ctx, `
			UPDATE idempotency_keys
			SET request_hash = $3, response_status = NULL, response_body = NULL, created_at = $4, expires_at = $5
			WHERE scope = $1 AND key = $2`,
			key.Scope, key.Value, key.RequestHash[:], now, now.Add(s.ttl))
		if err != nil {
			return nil, fmt.Errorf("renew expired idempotency key: %w", err)
		}
		return nil, nil
	}
	if !bytes.Equal(hash, key.RequestHash[:]) {
		return nil, apperr.New(apperr.CodeIdempotencyReused, nil)
	}
	if status == nil {
		// Claimed and committed without a response: only a caller that forgot Complete can
		// leave this state. Never redo the effect blindly.
		return nil, apperr.New(apperr.CodeOperationInProgress, nil)
	}
	return &Stored{Status: *status, Body: body}, nil
}

// Complete stores the response of a claimed key, in the same transaction as the effect.
func (s *Store) Complete(ctx context.Context, tx pgx.Tx, key Key, status int, body []byte) error {
	tag, err := tx.Exec(ctx, `
		UPDATE idempotency_keys SET response_status = $3, response_body = $4
		WHERE scope = $1 AND key = $2 AND request_hash = $5 AND response_status IS NULL`,
		key.Scope, key.Value, status, body, key.RequestHash[:])
	if err != nil {
		return fmt.Errorf("complete idempotency key: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("complete idempotency key %q: key not claimed in this transaction", key.Value)
	}
	return nil
}
