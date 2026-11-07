//go:build integration

package db_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db/dbtest"
)

var pg *dbtest.Postgres

func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }

func TestWithTxCommitsAndRollsBack(t *testing.T) {
	t.Parallel()
	pool := pg.NewDatabase(t)
	ctx := t.Context()
	if _, err := pool.Exec(ctx, "CREATE TABLE counters (id int PRIMARY KEY, n int NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if err := db.WithTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO counters VALUES (1, 0)")
		return err
	}); err != nil {
		t.Fatalf("commit: %v", err)
	}
	boom := errors.New("boom")
	err := db.WithTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "UPDATE counters SET n = 99 WHERE id = 1"); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	var n int
	if err := pool.QueryRow(ctx, "SELECT n FROM counters WHERE id = 1").Scan(&n); err != nil || n != 0 {
		t.Fatalf("n = %d (%v): the failed transaction must roll back", n, err)
	}
}

// MO3: N concurrent serializable read-modify-write increments all land exactly once thanks
// to the retry helper (without it, some fail with 40001).
func TestWithSerializableRetriesConflicts(t *testing.T) {
	t.Parallel()
	pool := pg.NewDatabase(t)
	ctx := t.Context()
	if _, err := pool.Exec(ctx, "CREATE TABLE counters (id int PRIMARY KEY, n int NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO counters VALUES (1, 0)"); err != nil {
		t.Fatal(err)
	}

	const workers = 4 // below SerializableAttempts: every worker must eventually win
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- db.WithSerializable(ctx, pool, func(tx pgx.Tx) error {
				var n int
				if err := tx.QueryRow(ctx, "SELECT n FROM counters WHERE id = 1").Scan(&n); err != nil {
					return err
				}
				_, err := tx.Exec(ctx, "UPDATE counters SET n = $1 WHERE id = 1", n+1)
				return err
			})
		}()
	}
	wg.Wait()
	close(errs)
	ok := 0
	for err := range errs {
		switch {
		case err == nil:
			ok++
		case apperr.IsCode(err, apperr.CodeOperationInProgress):
		default:
			t.Fatalf("serializable tx: %v", err)
		}
	}
	var n int
	if err := pool.QueryRow(ctx, "SELECT n FROM counters WHERE id = 1").Scan(&n); err != nil || n != ok {
		t.Fatalf("n = %d, successful transactions %d (%v): every committed increment counts once", n, ok, err)
	}
}

func TestWithSerializableGivesUpAsInProgress(t *testing.T) {
	t.Parallel()
	pool := pg.NewDatabase(t)
	ctx := t.Context()
	calls := 0
	err := db.WithSerializable(ctx, pool, func(tx pgx.Tx) error {
		calls++
		_, err := tx.Exec(ctx, "DO $$ BEGIN RAISE EXCEPTION 'conflict' USING ERRCODE = '40001'; END $$")
		return err
	})
	if !apperr.IsCode(err, apperr.CodeOperationInProgress) || calls != db.SerializableAttempts {
		t.Fatalf("err = %v after %d calls, want operation.in_progress after %d", err, calls, db.SerializableAttempts)
	}
}

func TestErrorPredicates(t *testing.T) {
	t.Parallel()
	pool := pg.NewDatabase(t)
	ctx := t.Context()
	if _, err := pool.Exec(ctx, "CREATE TABLE things (id int CONSTRAINT things_pk PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO things VALUES (1)"); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, "INSERT INTO things VALUES (1)")
	if !db.IsUniqueViolation(err, "things_pk") || !db.IsUniqueViolation(err, "") || db.IsUniqueViolation(err, "other") {
		t.Fatalf("IsUniqueViolation wrong for %v", err)
	}
	err = pool.QueryRow(ctx, "SELECT id FROM things WHERE id = 2").Scan(new(int))
	if !db.IsNoRows(err) {
		t.Fatalf("IsNoRows wrong for %v", err)
	}
}

func TestAdvisoryXactLockSerializes(t *testing.T) {
	t.Parallel()
	pool := pg.NewDatabase(t)
	ctx := t.Context()
	held, release := make(chan struct{}), make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- db.WithTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
			if err := db.AdvisoryXactLock(ctx, tx, "payer:1:ARS"); err != nil {
				return err
			}
			close(held)
			<-release
			return nil
		})
	}()
	<-held
	acquired := make(chan error, 1)
	go func() {
		acquired <- db.WithTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
			return db.AdvisoryXactLock(ctx, tx, "payer:1:ARS")
		})
	}()
	select {
	case err := <-acquired:
		t.Fatalf("second transaction got the lock while the first held it (err %v)", err)
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-acquired:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("lock never released")
	}
}
