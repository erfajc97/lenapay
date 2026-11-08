package realtime

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/clock"
	"lenapay/apps/api/internal/platform/httpx"
	platformrealtime "lenapay/apps/api/internal/platform/realtime"
)

// StreamHandler serves the SSE stream of the principal's org (R-IN-3). The guard resolved the
// caller's membership of {orgId}, so the stream only ever carries that org's events (T).
type StreamHandler struct {
	hub   *platformrealtime.Hub
	clock clock.Clock
}

// NewStreamHandler returns the stream handler.
func NewStreamHandler(hub *platformrealtime.Hub) *StreamHandler {
	return &StreamHandler{hub: hub, clock: clock.Real{}}
}

// ServeHTTP implements http.Handler.
func (h *StreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p, err := auth.RequirePrincipal(r.Context())
	if err == nil && !p.HasOrg() {
		err = apperr.New(apperr.CodeOrgNotFound, nil) // the route table always resolves the org
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	events, release := h.hub.Subscribe(p.OrgID) // before the headers: nothing is missed after connect
	defer release()
	if err := h.hub.StreamFrames(r.Context(), w, events, envelope(p.OrgID, h.clock)); err != nil {
		slog.WarnContext(r.Context(), "event stream ended", "err", err.Error())
	}
}

// envelope frames each event for both clients of the stream: an UNNAMED frame (EventSource
// delivers it as "message", which is what apps/mobile listens to) whose data is the contract's
// RealtimeEvent {type, orgId, occurredAt, data} with the members of data also lifted to the top
// level (so the app reads e.g. event.payin). Events whose data is not a JSON object are dropped.
func envelope(orgID uuid.UUID, clk clock.Clock) platformrealtime.Frame {
	return func(ev platformrealtime.Event) (string, []byte, bool) {
		var members map[string]json.RawMessage
		if err := json.Unmarshal(ev.Data, &members); err != nil || members == nil {
			return "", nil, false
		}
		out := make(map[string]any, len(members)+4)
		for k, v := range members {
			out[k] = v
		}
		out["type"], out["orgId"], out["occurredAt"], out["data"] = ev.Type, orgID, clk.Now().Format(time.RFC3339Nano), ev.Data
		b, err := json.Marshal(out)
		if err != nil {
			return "", nil, false
		}
		return "", b, true
	}
}
