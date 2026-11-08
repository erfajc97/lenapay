//go:build integration

package idempotency_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/clock"
	"lenapay/apps/api/internal/platform/db"
	"lenapay/apps/api/internal/platform/db/dbtest"
	"lenapay/apps/api/internal/platform/idempotency"
	"lenapay/apps/api/internal/platform/ids"
)

var pg *dbtest.Postgres

func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }

type payoutRequest struct {
	AccountID string `json:"accountId"`
	Minor     int64  `json:"minor"`
}

func mustHash(t *testing.T, v any) [32]byte {
	t.Helper()
	h, err := idempotency.HashRequest(v)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// effect simulates a money action guarded by the store: it inserts one row per effect.
func effect(ctx context.Context, pool *pgxpool.Pool, store *idempotency.Store, key idempotency.Key) (*idempotency.Stored, error) {
	var replay *idempotency.Stored
	err := db.WithTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		stored, err := store.Claim(ctx, tx, key)
		if err != nil || stored != nil {
			replay = stored
			return err
		}
		if _, err := tx.Exec(ctx, "INSERT INTO effects (key) VALUES ($1)", key.Value); err != nil {
			return err
		}
		return store.Complete(ctx, tx, key, 201, []byte(`{"id":"p1"}`))
	})
	return replay, err
}

func setup(t *testing.T) (*pgxpool.Pool, *idempotency.Store, *clock.Fake) {
	t.Helper()
	pool := pg.NewDatabase(t)
	if _, err := pool.Exec(t.Context(), "CREATE TABLE effects (id serial PRIMARY KEY, key text NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	fake := clock.NewFake(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	return pool, idempotency.NewStore(fake, idempotency.DefaultTTL), fake
}

func countEffects(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM effects").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// MO2: the same key + same payload replays the stored response with ONE effect; the same key
// + another payload is 409 idempotency.key_reused; scopes are per tenant.
func TestReplayAndMismatch(t *testing.T) {
	t.Parallel()
	pool, store, _ := setup(t)
	ctx := t.Context()
	key := idempotency.Key{
		Scope:       idempotency.Scope(ids.New(), "payouts.create"),
		Value:       "client-key-0000000001",
		RequestHash: mustHash(t, payoutRequest{AccountID: "a1", Minor: 1000}),
	}

	if replay, err := effect(ctx, pool, store, key); err != nil || replay != nil {
		t.Fatalf("first call: replay=%v err=%v", replay, err)
	}
	replay, err := effect(ctx, pool, store, key)
	if err != nil || replay == nil || replay.Status != 201 || string(replay.Body) != `{"id":"p1"}` {
		t.Fatalf("second call must replay: replay=%+v err=%v", replay, err)
	}
	if n := countEffects(t, pool); n != 1 {
		t.Fatalf("effects = %d, want 1", n)
	}

	other := key
	other.RequestHash = mustHash(t, payoutRequest{AccountID: "a1", Minor: 1001})
	if _, err := effect(ctx, pool, store, other); !apperr.IsCode(err, apperr.CodeIdempotencyReused) {
		t.Fatalf("different payload err = %v, want idempotency.key_reused", err)
	}

	foreign := key
	foreign.Scope = idempotency.Scope(ids.New(), "payouts.create")
	if replay, err := effect(ctx, pool, store, foreign); err != nil || replay != nil {
		t.Fatalf("other scope must be a fresh effect: replay=%v err=%v", replay, err)
	}
	if n := countEffects(t, pool); n != 2 {
		t.Fatalf("effects = %d, want 2", n)
	}
}

// A failed effect rolls the claim back with it: the client can retry the same key.
func TestRolledBackClaimIsForgotten(t *testing.T) {
	t.Parallel()
	pool, store, _ := setup(t)
	ctx := t.Context()
	key := idempotency.Key{Scope: "org:x:op", Value: "client-key-0000000002", RequestHash: mustHash(t, 1)}
	err := db.WithTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if _, err := store.Claim(ctx, tx, key); err != nil {
			return err
		}
		return apperr.New(apperr.CodeInsufficientFunds, nil)
	})
	if !apperr.IsCode(err, apperr.CodeInsufficientFunds) {
		t.Fatalf("err = %v", err)
	}
	if replay, err := effect(ctx, pool, store, key); err != nil || replay != nil {
		t.Fatalf("retry after rollback: replay=%v err=%v", replay, err)
	}
}

func TestExpiredKeyIsReusable(t *testing.T) {
	t.Parallel()
	pool, store, fake := setup(t)
	ctx := t.Context()
	key := idempotency.Key{Scope: "org:x:op", Value: "client-key-0000000003", RequestHash: mustHash(t, 1)}
	if _, err := effect(ctx, pool, store, key); err != nil {
		t.Fatal(err)
	}
	// Boundary pair (FT1): one second before expiry still replays, at expiry it is fresh.
	fake.Advance(idempotency.DefaultTTL - time.Second)
	if replay, err := effect(ctx, pool, store, key); err != nil || replay == nil {
		t.Fatalf("before expiry must replay: replay=%v err=%v", replay, err)
	}
	fake.Advance(time.Second)
	other := key
	other.RequestHash = mustHash(t, 2)
	if replay, err := effect(ctx, pool, store, other); err != nil || replay != nil {
		t.Fatalf("expired key must be fresh: replay=%v err=%v", replay, err)
	}
}

func TestClaimWithoutCompleteIsInProgress(t *testing.T) {
	t.Parallel()
	pool, store, _ := setup(t)
	ctx := t.Context()
	key := idempotency.Key{Scope: "org:x:op", Value: "client-key-0000000004", RequestHash: mustHash(t, 1)}
	if err := db.WithTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err := store.Claim(ctx, tx, key)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := effect(ctx, pool, store, key); !apperr.IsCode(err, apperr.CodeOperationInProgress) {
		t.Fatalf("err = %v, want operation.in_progress", err)
	}
}

func TestCompleteRequiresClaim(t *testing.T) {
	t.Parallel()
	pool, store, _ := setup(t)
	ctx := t.Context()
	key := idempotency.Key{Scope: "org:x:op", Value: "client-key-0000000005", RequestHash: mustHash(t, 1)}
	if err := db.WithTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return store.Complete(ctx, tx, key, 201, nil)
	}); err == nil {
		t.Fatal("Complete without Claim must fail")
	}
}

// MO6: N parallel requests with the same key produce exactly one effect; the rest replay.
func TestConcurrentSameKeyOneEffect(t *testing.T) {
	t.Parallel()
	pool, store, _ := setup(t)
	ctx := t.Context()
	key := idempotency.Key{Scope: "org:x:op", Value: "client-key-0000000006", RequestHash: mustHash(t, 1)}

	const n = 8
	var wg sync.WaitGroup
	results := make(chan error, n)
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := effect(ctx, pool, store, key)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("concurrent call: %v", err)
		}
	}
	if got := countEffects(t, pool); got != 1 {
		t.Fatalf("effects = %d, want exactly 1", got)
	}
}
