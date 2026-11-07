//go:build integration

package realtime_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/db"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db/dbtest"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/realtime"
)

var pg *dbtest.Postgres

func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }

// Events travel through Postgres only when the publishing transaction commits.
func TestPublishDeliversOnlyCommittedEvents(t *testing.T) {
	t.Parallel()
	pool := pg.NewDatabase(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	hub := realtime.NewHub(pool)
	go func() { _ = hub.Run(ctx) }()

	org := uuid.New()
	events, release := hub.Subscribe(org)
	defer release()

	publish := func(typ string, commit bool) {
		err := db.WithTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
			if err := realtime.Publish(ctx, tx, org, realtime.Event{Type: typ, Data: json.RawMessage(`{}`)}); err != nil {
				return err
			}
			if !commit {
				return errors.New("rollback")
			}
			return nil
		})
		if commit && err != nil {
			t.Fatal(err)
		}
	}

	// The listener needs a moment to LISTEN: publish the committed event until it lands.
	deadline := time.Now().Add(10 * time.Second)
	for got := false; !got; {
		publish("committed", true)
		select {
		case ev := <-events:
			if ev.Type != "committed" {
				t.Fatalf("event %+v", ev)
			}
			got = true
		case <-time.After(200 * time.Millisecond):
			if time.Now().After(deadline) {
				t.Fatal("listener never received an event")
			}
		}
	}
	for len(events) > 0 { // drain duplicates from the retry loop
		<-events
	}
	publish("rolled_back", false)
	publish("second", true)
	select {
	case ev := <-events:
		if ev.Type != "second" {
			t.Fatalf("got %q: a rolled-back event was delivered", ev.Type)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("committed event not delivered")
	}
}

func TestPublishRejectsOversizedPayload(t *testing.T) {
	t.Parallel()
	pool := pg.NewDatabase(t)
	ctx := t.Context()
	big := make([]byte, 8000)
	for i := range big {
		big[i] = 'x'
	}
	data, _ := json.Marshal(string(big))
	err := db.WithTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return realtime.Publish(ctx, tx, uuid.New(), realtime.Event{Type: "x", Data: data})
	})
	if err == nil {
		t.Fatal("oversized payload must be rejected")
	}
}
