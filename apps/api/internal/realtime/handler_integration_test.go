//go:build integration

package realtime

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/db"
	"lenapay/apps/api/internal/platform/db/dbtest"
	platformrealtime "lenapay/apps/api/internal/platform/realtime"
)

var pg *dbtest.Postgres

func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }

// A committed event of the caller's org reaches its stream as an unnamed frame; another org's
// event never does (T: the stream is keyed by the principal's org only).
func TestStreamCarriesOnlyTheCallersOrg(t *testing.T) {
	t.Parallel()
	pool := pg.NewDatabase(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	hub := platformrealtime.NewHub(pool)
	go func() { _ = hub.Run(ctx) }()

	mine, other := uuid.New(), uuid.New()
	h := NewStreamHandler(hub)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), auth.Principal{UserID: uuid.New(), OrgID: mine})))
	}))
	defer srv.Close()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	lines := make(chan string, 64)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()

	publish := func(org uuid.UUID, id string) {
		err := db.WithTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
			return platformrealtime.Publish(ctx, tx, org, platformrealtime.Event{Type: "payin.credited", Data: json.RawMessage(`{"payinId":"` + id + `"}`)})
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.After(10 * time.Second)
	for {
		publish(other, "foreign")
		publish(mine, "own")
		select {
		case line := <-lines:
			if strings.HasPrefix(line, "event:") || strings.Contains(line, "foreign") {
				t.Fatalf("line %q", line)
			}
			if strings.HasPrefix(line, "data:") && strings.Contains(line, `"payinId":"own"`) && strings.Contains(line, mine.String()) {
				return
			}
		case <-deadline:
			t.Fatal("no frame within 10s")
		case <-time.After(200 * time.Millisecond): // the listener may not be LISTENing yet
		}
	}
}
