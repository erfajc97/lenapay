// Package db owns the pgx pool, the ONE transaction helper and the serializable retry
// (AG9, AG11, MO3), lock helpers and migrations.
package db

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
)

// NewPool opens the pool with server-side timeouts, so a stuck query or an idle transaction
// cannot hold a connection (and its locks) forever.
func NewPool(ctx context.Context, url string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("db: parse url: %w", err) // pgx never puts the password in this error
	}
	cfg.MaxConns = maxConns
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "5s"
	cfg.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = "10s"
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return pool, nil
}

// TxStarter is what WithTx needs: *pgxpool.Pool, or a pgx.Tx (savepoint) in tests.
type TxStarter interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
}

// WithTx runs fn in ONE transaction: commit when fn returns nil, rollback otherwise (and on
// panic). fn must not keep tx after it returns and must not call external services: side
// effects go to River with InsertTx (same transaction) and run after commit.
func WithTx(ctx context.Context, db TxStarter, opts pgx.TxOptions, fn func(pgx.Tx) error) error {
	return pgx.BeginTxFunc(ctx, db, opts, fn)
}

// SerializableAttempts bounds WithSerializable.
const SerializableAttempts = 5

// WithSerializable runs fn in a SERIALIZABLE transaction and retries it on a serialization
// failure or deadlock (40001, 40P01) with jittered backoff. fn is re-run from scratch, so it
// must decide everything from what it reads inside the transaction. On giving up it returns
// operation.in_progress (409): the client retries with the same idempotency key (MO3).
func WithSerializable(ctx context.Context, db TxStarter, fn func(pgx.Tx) error) error {
	opts := pgx.TxOptions{IsoLevel: pgx.Serializable}
	for attempt := 1; ; attempt++ {
		err := WithTx(ctx, db, opts, fn)
		if err == nil || !IsRetryable(err) {
			return err
		}
		if attempt == SerializableAttempts {
			return apperr.Wrap(apperr.CodeOperationInProgress, err, nil)
		}
		backoff := time.Duration(attempt*attempt)*10*time.Millisecond + rand.N(10*time.Millisecond) //nolint:gosec // jitter, not a secret
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
}

// IsRetryable reports a serialization failure or a deadlock.
func IsRetryable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == "40001" || pgErr.Code == "40P01")
}

// IsUniqueViolation reports a unique violation of that constraint ("" = any).
func IsUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" &&
		(constraint == "" || pgErr.ConstraintName == constraint)
}

// IsNoRows reports a query that found nothing (map it to the feature's not_found code).
func IsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// SortedIDs returns a sorted, de-duplicated copy of ids. Multi-row locks take rows in this
// order (one global lock order), so two writers never deadlock on the same pair.
func SortedIDs(ids []uuid.UUID) []uuid.UUID {
	out := slices.Clone(ids)
	slices.SortFunc(out, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	return slices.Compact(out)
}

// AdvisoryXactLock takes a transaction-scoped advisory lock on key (released at commit or
// rollback): it serializes work on something that has no row yet (e.g. "first CVU of this
// payer in ARS").
func AdvisoryXactLock(ctx context.Context, tx pgx.Tx, key string) error {
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", key); err != nil {
		return fmt.Errorf("db: advisory lock: %w", err)
	}
	return nil
}
