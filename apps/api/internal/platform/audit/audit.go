// Package audit appends to the audit log (R-NF-3, T8): who did what to which target, with the
// request id and idempotency key, in the SAME transaction as the action. The log is
// insert-only (a trigger rejects UPDATE/DELETE); payloads are stored as a hash, never raw.
package audit

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	applog "github.com/erfajc97/lenapay/apps/api/internal/platform/log"
)

// ActorKind says who acted.
type ActorKind string

// Actor kinds (DB CHECK on audit_log.actor_kind).
const (
	ActorUser     ActorKind = "user"
	ActorSystem   ActorKind = "system"
	ActorInternal ActorKind = "internal"
	ActorPSP      ActorKind = "psp"
)

// Entry is one audited action.
type Entry struct {
	OrgID          *uuid.UUID
	ActorID        *uuid.UUID
	ActorKind      ActorKind
	Action         string // e.g. "payout.create"
	TargetType     string // e.g. "payout"
	TargetID       *uuid.UUID
	IdempotencyKey string
	// Payload is hashed (sha256 of its JSON); keep secrets out of it anyway.
	Payload any
}

// Logger writes audit entries.
type Logger struct {
	clock clock.Clock
}

// NewLogger returns an audit logger.
func NewLogger(c clock.Clock) *Logger {
	return &Logger{clock: c}
}

// Record appends e inside tx. The request id comes from ctx.
func (l *Logger) Record(ctx context.Context, tx pgx.Tx, e Entry) error {
	var payloadHash []byte
	if e.Payload != nil {
		b, err := json.Marshal(e.Payload)
		if err != nil {
			return fmt.Errorf("audit payload: %w", err)
		}
		sum := sha256.Sum256(b)
		payloadHash = sum[:]
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO audit_log (id, org_id, actor_id, actor_kind, action, target_type, target_id,
		                       request_id, idempotency_key, payload_hash, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), NULLIF($9, ''), $10, $11)`,
		ids.New(), e.OrgID, e.ActorID, string(e.ActorKind), e.Action, e.TargetType, e.TargetID,
		applog.RequestID(ctx), e.IdempotencyKey, payloadHash, l.clock.Now())
	if err != nil {
		return fmt.Errorf("audit %s: %w", e.Action, err)
	}
	return nil
}
