package realtime

import (
	"net/http"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/httpx"
	platformrealtime "github.com/erfajc97/lenapay/apps/api/internal/platform/realtime"
)

// StreamHandler serves the SSE stream of the principal's org. It answers 501 until lane C
// subscribes to the hub with auth.PrincipalFrom(ctx).OrgID and calls Hub.Stream.
type StreamHandler struct {
	hub *platformrealtime.Hub
}

// NewStreamHandler returns the stream handler.
func NewStreamHandler(hub *platformrealtime.Hub) *StreamHandler { return &StreamHandler{hub: hub} }

// ServeHTTP implements http.Handler.
func (h *StreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = h.hub
	httpx.WriteError(w, r, apperr.NotImplemented())
}
