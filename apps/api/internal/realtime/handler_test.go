package realtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	platformrealtime "github.com/erfajc97/lenapay/apps/api/internal/platform/realtime"
)

// The frame serves both clients: the contract's RealtimeEvent keys and, lifted, the members
// the app reads (event.payin); unnamed, so EventSource delivers it as "message".
func TestEnvelopeFrame(t *testing.T) {
	t.Parallel()
	org := uuid.New()
	clk := clock.NewFake(time.Date(2026, 11, 7, 15, 0, 0, 0, time.UTC))
	name, data, ok := envelope(org, clk)(platformrealtime.Event{
		Type: "payin.credited", Data: json.RawMessage(`{"payinId":"p1","payin":{"id":"p1","amount":150000}}`),
	})
	if !ok || name != "" {
		t.Fatalf("name %q ok %v", name, ok)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	inner := got["data"].(map[string]any)
	if got["type"] != "payin.credited" || got["orgId"] != org.String() || got["occurredAt"] != "2026-11-07T15:00:00Z" ||
		inner["payinId"] != "p1" || got["payin"].(map[string]any)["amount"] != float64(150000) || got["payinId"] != "p1" || len(got) != 6 {
		t.Fatalf("frame %v", got)
	}
	if _, _, ok := envelope(org, clk)(platformrealtime.Event{Type: "x", Data: json.RawMessage(`[1]`)}); ok {
		t.Fatal("non-object data must be dropped")
	}
}

func TestStreamWithoutOrgIsRefused(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	NewStreamHandler(platformrealtime.NewHub(nil)).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/orgs/x/events", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}
