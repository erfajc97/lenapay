//go:build integration

package notifications_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/erfajc97/lenapay/apps/api/internal/notifications"
	"github.com/erfajc97/lenapay/apps/api/internal/notifications/provider/fake"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db/dbtest"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
)

var pg *dbtest.Postgres

func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }

type recorder struct {
	mu   sync.Mutex
	jobs []notifications.DeliverArgs
}

func (r *recorder) InsertTx(_ context.Context, _ pgx.Tx, args river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs = append(r.jobs, args.(notifications.DeliverArgs))
	return &rivertype.JobInsertResult{}, nil
}

type env struct {
	pool   *pgxpool.Pool
	clk    *clock.Fake
	svc    notifications.Service
	sender *fake.Sender
	jobs   *recorder
}

func setup(t *testing.T) env {
	t.Helper()
	pool := pg.NewDatabase(t)
	clk := clock.NewFake(time.Date(2026, 11, 7, 15, 0, 0, 0, time.UTC))
	e := env{pool: pool, clk: clk, sender: fake.NewSender(true), jobs: &recorder{}}
	e.svc = notifications.NewService(notifications.Deps{Pool: pool, Clock: clk, Jobs: e.jobs, Sender: e.sender})
	return e
}

func (e env) user(t *testing.T) uuid.UUID {
	t.Helper()
	id := ids.New()
	if _, err := e.pool.Exec(t.Context(), `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, 'x')`, id, id.String()+"@t.local"); err != nil {
		t.Fatal(err)
	}
	return id
}

func (e env) status(t *testing.T, id uuid.UUID) (string, int) {
	t.Helper()
	var status string
	var attempts int
	if err := e.pool.QueryRow(t.Context(), `SELECT status, attempts FROM notifications WHERE id = $1`, id).Scan(&status, &attempts); err != nil {
		t.Fatal(err)
	}
	return status, attempts
}

func (e env) notify(t *testing.T, users ...uuid.UUID) {
	t.Helper()
	err := db.WithTx(t.Context(), e.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return e.svc.Notify(t.Context(), tx, notifications.Notification{UserIDs: users, Kind: "payin.credited", Data: map[string]string{"payinId": "p1"}})
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPushTokensMoveBetweenUsers(t *testing.T) {
	t.Parallel()
	e := setup(t)
	ana, beto := e.user(t), e.user(t)
	ctx := t.Context()
	if err := e.svc.RegisterPushToken(ctx, auth.Principal{UserID: ana}, "ExponentPushToken[abc]", notifications.PlatformIOS); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.RegisterPushToken(ctx, auth.Principal{UserID: beto}, "ExponentPushToken[abc]", notifications.PlatformAndroid); err != nil {
		t.Fatal(err)
	}
	var owner uuid.UUID
	var platform string
	if err := e.pool.QueryRow(ctx, `SELECT user_id, platform FROM push_tokens WHERE token = 'ExponentPushToken[abc]'`).Scan(&owner, &platform); err != nil || owner != beto || platform != "android" {
		t.Fatalf("token owner %s %s %v", owner, platform, err)
	}
	if err := e.svc.RegisterPushToken(ctx, auth.Principal{UserID: ana}, "", notifications.PlatformIOS); !apperr.IsCode(err, apperr.CodeValidation) {
		t.Fatalf("empty token: %v", err)
	}
	// web and unknown (the contract default when the client does not say) are stored as given.
	for _, pl := range []notifications.Platform{notifications.PlatformWeb, notifications.PlatformUnknown} {
		if err := e.svc.RegisterPushToken(ctx, auth.Principal{UserID: ana}, "ExponentPushToken["+string(pl)+"]", pl); err != nil {
			t.Fatalf("%s: %v", pl, err)
		}
	}
	if err := e.svc.RegisterPushToken(ctx, auth.Principal{UserID: ana}, "ExponentPushToken[x]", "symbian"); !apperr.IsCode(err, apperr.CodeValidation) {
		t.Fatalf("unknown platform: %v", err)
	}
}

// The outbox entry and its job are written in the fact's transaction: a rollback leaves neither.
func TestNotifyIsTransactional(t *testing.T) {
	t.Parallel()
	e := setup(t)
	ana, beto := e.user(t), e.user(t)
	e.notify(t, ana, beto)
	if len(e.jobs.jobs) != 2 {
		t.Fatalf("jobs = %d", len(e.jobs.jobs))
	}
	err := db.WithTx(t.Context(), e.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := e.svc.Notify(t.Context(), tx, notifications.Notification{UserIDs: []uuid.UUID{ana}, Kind: "payin.credited"}); err != nil {
			return err
		}
		return errors.New("the fact failed")
	})
	if err == nil {
		t.Fatal("expected rollback")
	}
	var n int
	_ = e.pool.QueryRow(t.Context(), `SELECT count(*) FROM notifications`).Scan(&n)
	if n != 2 {
		t.Fatalf("rolled-back notification persisted: %d", n)
	}
}

// Delivery is at least once and idempotent: a sent entry is not pushed again; a failing send
// counts attempts until the entry is given up; a user without devices is done at once.
func TestDeliver(t *testing.T) {
	t.Parallel()
	e := setup(t)
	ctx := t.Context()
	ana, beto := e.user(t), e.user(t)
	if err := e.svc.RegisterPushToken(ctx, auth.Principal{UserID: ana}, "ExponentPushToken[ana]", notifications.PlatformIOS); err != nil {
		t.Fatal(err)
	}
	e.notify(t, ana, beto)
	toAna, toBeto := e.jobs.jobs[0].NotificationID, e.jobs.jobs[1].NotificationID
	for range 2 {
		if err := e.svc.Deliver(ctx, toAna); err != nil {
			t.Fatal(err)
		}
	}
	pushes := e.sender.Pushes()
	if len(pushes) != 1 || pushes[0].Tokens[0] != "ExponentPushToken[ana]" || pushes[0].Data["payinId"] != "p1" || pushes[0].Kind != "payin.credited" {
		t.Fatalf("pushes %+v", pushes)
	}
	if status, attempts := e.status(t, toAna); status != "sent" || attempts != 1 {
		t.Fatalf("ana %s %d", status, attempts)
	}
	if err := e.svc.Deliver(ctx, toBeto); err != nil || len(e.sender.Pushes()) != 1 {
		t.Fatalf("no devices: %v", err)
	}
	if status, _ := e.status(t, toBeto); status != "sent" {
		t.Fatal("an entry without devices is done")
	}

	e.notify(t, ana)
	failing := e.jobs.jobs[2].NotificationID
	boom := errors.New("expo down")
	for i := 1; i <= 10; i++ {
		e.sender.FailNext(boom)
		if err := e.svc.Deliver(ctx, failing); !errors.Is(err, boom) {
			t.Fatalf("attempt %d: %v", i, err)
		}
		status, attempts := e.status(t, failing)
		want := "pending"
		if i == 10 { // the last allowed attempt gives up
			want = "failed"
		}
		if status != want || attempts != i {
			t.Fatalf("attempt %d: %s %d", i, status, attempts)
		}
	}
	if err := e.svc.Deliver(ctx, uuid.New()); err != nil {
		t.Fatalf("a deleted entry is not an error: %v", err)
	}
}

func TestRequeueStale(t *testing.T) {
	t.Parallel()
	e := setup(t)
	e.notify(t, e.user(t))
	e.jobs.jobs = nil
	if n, err := e.svc.RequeueStale(t.Context(), 15*time.Minute); err != nil || n != 0 {
		t.Fatalf("fresh entries are left to their job: %d %v", n, err)
	}
	if _, err := e.pool.Exec(t.Context(), `UPDATE notifications SET created_at = $1`, e.clk.Now().Add(-16*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if n, err := e.svc.RequeueStale(t.Context(), 15*time.Minute); err != nil || n != 1 || len(e.jobs.jobs) != 1 {
		t.Fatalf("stale: %d %v", n, err)
	}
}

func TestEmails(t *testing.T) {
	t.Parallel()
	e := setup(t)
	if err := e.svc.SendEmailCode(t.Context(), "ana@example.com", "123456"); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.SendInvite(t.Context(), "beto@example.com", "Desarrollos Norte SA", "tok"); err != nil {
		t.Fatal(err)
	}
	emails := e.sender.Emails()
	if len(emails) != 2 || emails[0].Template != notifications.TemplateEmailCode || emails[0].Data["code"] != "123456" ||
		emails[1].Template != notifications.TemplateInvite || emails[1].Data["token"] != "tok" || emails[1].To != "beto@example.com" {
		t.Fatalf("emails %+v", emails)
	}
}
