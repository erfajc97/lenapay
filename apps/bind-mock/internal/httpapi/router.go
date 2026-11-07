// Package httpapi is the HTTP surface of the fake PSP (lane C, backlog C1): BIND-shaped
// endpoints (CVU create / alias / disable, transfers, transfer by id, destination resolve,
// master balance, statement), the /_mock control endpoints (simulate an incoming transfer,
// force a payout failure or timeout) and the signed webhook sender. E0 ships /healthz only.
package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// NewRouter returns the bind-mock routes.
func NewRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
	})
	return mux
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.WarnContext(r.Context(), "write response", "err", err.Error())
	}
}
