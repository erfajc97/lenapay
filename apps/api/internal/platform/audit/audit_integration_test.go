//go:build integration

package audit_test

import (
	"context"
	"crypto/sha256"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"lenapay/apps/api/internal/platform/audit"
	"lenapay/apps/api/internal/platform/clock"
	"lenapay/apps/api/internal/platform/db"
	"lenapay/apps/api/internal/platform/db/dbtest"
	"lenapay/apps/api/internal/platform/ids"
	applog "lenapay/apps/api/internal/platform/log"
)

var pg *dbtest.Postgres

func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }

func TestRecordAppendsWithRequestContext(t *testing.T) {
	t.Parallel()
	pool := pg.NewDatabase(t)
	ctx := applog.WithRequestID(t.Context(), "req-audit-0001")
	at := time.Date(2026, 11, 7, 15, 0, 0, 0, time.UTC)
	logger := audit.NewLogger(clock.NewFake(at))
	target := ids.New()

	err := db.WithTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return logger.Record(ctx, tx, audit.Entry{
			ActorKind:      audit.ActorSystem,
			Action:         "payout.create",
			TargetType:     "payout",
			TargetID:       &target,
			IdempotencyKey: "client-key-0000000001",
			Payload:        map[string]int64{"minor": 1000},
		})
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	var (
		action, requestID, key string
		hash                   []byte
		createdAt              time.Time
	)
	if err := pool.QueryRow(ctx, `SELECT action, request_id, idempotency_key, payload_hash, created_at
		FROM audit_log WHERE target_id = $1`, target).Scan(&action, &requestID, &key, &hash, &createdAt); err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256([]byte(`{"minor":1000}`))
	if action != "payout.create" || requestID != "req-audit-0001" || key != "client-key-0000000001" ||
		string(hash) != string(want[:]) || !createdAt.Equal(at) {
		t.Fatalf("row = %s %s %s %x %v", action, requestID, key, hash, createdAt)
	}
}

// T8: the audit log is append-only at the database level.
func TestAuditLogIsAppendOnly(t *testing.T) {
	t.Parallel()
	pool := pg.NewDatabase(t)
	ctx := t.Context()
	logger := audit.NewLogger(clock.Real{})
	if err := db.WithTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return logger.Record(ctx, tx, audit.Entry{ActorKind: audit.ActorSystem, Action: "x.y", TargetType: "x"})
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE audit_log SET action = 'tampered'"); err == nil {
		t.Fatal("UPDATE on audit_log must fail")
	}
	if _, err := pool.Exec(ctx, "DELETE FROM audit_log"); err == nil {
		t.Fatal("DELETE on audit_log must fail")
	}
}
