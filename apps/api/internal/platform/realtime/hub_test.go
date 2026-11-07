package realtime

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func waitEvent(t *testing.T, ch <-chan Event) Event {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	case <-time.After(2 * time.Second):
		t.Fatal("no event received")
		return Event{}
	}
}

// T: an event reaches only the subscribers of its own org.
func TestDispatchIsPerOrg(t *testing.T) {
	t.Parallel()
	h := NewHub(nil)
	orgA, orgB := uuid.New(), uuid.New()
	chA, releaseA := h.Subscribe(orgA)
	defer releaseA()
	chB, releaseB := h.Subscribe(orgB)
	defer releaseB()

	h.dispatch(envelope{OrgID: orgA, Event: Event{Type: "payin.credited", Data: json.RawMessage(`{"id":"p1"}`)}})
	if ev := waitEvent(t, chA); ev.Type != "payin.credited" || string(ev.Data) != `{"id":"p1"}` {
		t.Fatalf("event %+v", ev)
	}
	select {
	case ev := <-chB:
		t.Fatalf("org B received org A's event: %+v", ev)
	default:
	}
}

func TestReleaseIsIdempotentAndStopsDelivery(t *testing.T) {
	t.Parallel()
	h := NewHub(nil)
	org := uuid.New()
	ch, release := h.Subscribe(org)
	release()
	release()
	h.dispatch(envelope{OrgID: org, Event: Event{Type: "x", Data: json.RawMessage(`{}`)}})
	select {
	case ev := <-ch:
		t.Fatalf("released subscriber got %+v", ev)
	default:
	}
}

// A slow client never blocks the listener: extra events are dropped for that client only.
func TestSlowSubscriberDoesNotBlock(t *testing.T) {
	t.Parallel()
	h := NewHub(nil)
	org := uuid.New()
	_, release := h.Subscribe(org)
	defer release()
	done := make(chan struct{})
	go func() {
		for range bufferPerConn + 5 {
			h.dispatch(envelope{OrgID: org, Event: Event{Type: "x", Data: json.RawMessage(`{}`)}})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch blocked on a slow subscriber")
	}
}

func TestStreamWritesEvents(t *testing.T) {
	t.Parallel()
	h := NewHub(nil)
	events := make(chan Event, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = h.Stream(r.Context(), w, events)
	}))
	defer srv.Close()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("content type %q", resp.Header.Get("Content-Type"))
	}
	events <- Event{Type: "payin.credited", Data: json.RawMessage(`{"id":"p"}`)}
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(line, "event: payin.credited") {
			data, _ := reader.ReadString('\n')
			if data != "data: {\"id\":\"p\"}\n" {
				t.Fatalf("data line %q", data)
			}
			return
		}
	}
}
