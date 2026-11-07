// Package realtime pushes per-organization events (R-IN-3). Producers call Publish INSIDE the
// transaction of the change: Postgres delivers pg_notify only at COMMIT, so a rolled-back
// change never notifies. ONE listener connection per process fans notifications out to the
// SSE streams subscribed to that org. Payloads carry ids only: clients refetch through the
// authorized endpoints, and delivery is best effort (anything that must not be lost is a job).
package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Channel is the Postgres NOTIFY channel shared by every org.
const Channel = "lenapay_events"

const (
	maxPayload     = 7000 // under Postgres' 8000-byte NOTIFY limit
	bufferPerConn  = 32
	heartbeatEvery = 15 * time.Second
)

// Event is one server-sent event of an org.
type Event struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type envelope struct {
	OrgID uuid.UUID `json:"orgId"`
	Event Event     `json:"event"`
}

// Publish queues ev for orgID inside tx; Postgres delivers it at commit (never on rollback).
func Publish(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, ev Event) error {
	payload, err := json.Marshal(envelope{OrgID: orgID, Event: ev})
	if err != nil {
		return fmt.Errorf("realtime: publish: %w", err)
	}
	if len(payload) > maxPayload {
		return fmt.Errorf("realtime: payload of %d bytes exceeds %d", len(payload), maxPayload)
	}
	if _, err := tx.Exec(ctx, "SELECT pg_notify($1, $2)", Channel, string(payload)); err != nil {
		return fmt.Errorf("realtime: notify: %w", err)
	}
	return nil
}

// Hub fans notifications out to in-process subscribers keyed by org.
type Hub struct {
	pool *pgxpool.Pool
	mu   sync.Mutex
	subs map[uuid.UUID]map[chan Event]struct{}
	done chan struct{} // closed when Run returns: open streams end, so shutdown is not held
}

// NewHub returns a hub; run it with Run in the process errgroup.
func NewHub(pool *pgxpool.Pool) *Hub {
	return &Hub{pool: pool, subs: map[uuid.UUID]map[chan Event]struct{}{}, done: make(chan struct{})}
}

// Run listens until ctx ends, reconnecting with capped backoff. It returns nil on shutdown.
func (h *Hub) Run(ctx context.Context) error {
	defer close(h.done)
	backoff := 500 * time.Millisecond
	for {
		err := h.listen(ctx)
		if ctx.Err() != nil {
			return nil
		}
		slog.WarnContext(ctx, "realtime listener stopped; reconnecting", "err", err.Error(), "in", backoff.String())
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

func (h *Hub) listen(ctx context.Context) error {
	conn, err := h.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire: %w", err)
	}
	// LISTEN state lives on the connection: take it out of the pool for good.
	listener := conn.Hijack()
	defer func() { _ = listener.Close(context.WithoutCancel(ctx)) }() // the connection is discarded either way
	if _, err := listener.Exec(ctx, "LISTEN "+Channel); err != nil {  // a constant: never user text
		return fmt.Errorf("listen: %w", err)
	}
	for {
		n, err := listener.WaitForNotification(ctx)
		if err != nil {
			return fmt.Errorf("wait: %w", err)
		}
		var env envelope
		if err := json.Unmarshal([]byte(n.Payload), &env); err != nil {
			slog.WarnContext(ctx, "realtime: malformed notification dropped", "err", err.Error())
			continue
		}
		h.dispatch(env)
	}
}

func (h *Hub) dispatch(env envelope) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[env.OrgID] {
		select {
		case ch <- env.Event:
		default: // slow consumer: drop; the client refetches on its next event or reconnect
		}
	}
}

// Subscribe registers a stream for orgID. Call release exactly once when the stream ends.
func (h *Hub) Subscribe(orgID uuid.UUID) (events <-chan Event, release func()) {
	ch := make(chan Event, bufferPerConn)
	h.mu.Lock()
	if h.subs[orgID] == nil {
		h.subs[orgID] = map[chan Event]struct{}{}
	}
	h.subs[orgID][ch] = struct{}{}
	h.mu.Unlock()
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.subs[orgID], ch)
			if len(h.subs[orgID]) == 0 {
				delete(h.subs, orgID)
			}
			h.mu.Unlock()
		})
	}
}

// Stream writes events to w as text/event-stream until the client leaves or the hub stops,
// with a comment heartbeat so proxies keep the connection open. It lifts the server's write
// deadline for this response only.
func (h *Hub) Stream(ctx context.Context, w http.ResponseWriter, events <-chan Event) error {
	return h.StreamFrames(ctx, w, events, namedFrame)
}

// Frame renders one event as an SSE frame: its name ("" = an unnamed frame, which an
// EventSource delivers as "message") and its data line. ok=false skips the event.
type Frame func(ev Event) (name string, data []byte, ok bool)

// namedFrame is the default frame: "event: <type>" and the event's data.
func namedFrame(ev Event) (string, []byte, bool) { return ev.Type, ev.Data, true }

// StreamFrames is Stream with a custom frame (the org stream adds the envelope clients read).
func (h *Hub) StreamFrames(ctx context.Context, w http.ResponseWriter, events <-chan Event, frame Frame) error {
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return fmt.Errorf("realtime: lift write deadline: %w", err)
	}
	hd := w.Header()
	hd.Set("Content-Type", "text/event-stream")
	hd.Set("Cache-Control", "no-store")
	hd.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if err := rc.Flush(); err != nil {
		return fmt.Errorf("realtime: flush: %w", err)
	}

	heartbeat := time.NewTicker(heartbeatEvery)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-h.done:
			return nil
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return nil // client gone
			}
		case ev := <-events:
			name, data, ok := frame(ev)
			if !ok {
				continue
			}
			if name != "" {
				if _, err := fmt.Fprintf(w, "event: %s\n", name); err != nil {
					return nil // client gone
				}
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return nil // client gone
			}
		}
		if err := rc.Flush(); err != nil {
			return nil // client gone
		}
	}
}
