// Package log configures log/slog once: JSON to stdout, the request id from the context on
// every line, and sensitive keys redacted whatever the caller passes (A15, AG17).
package log

import (
	"context"
	"io"
	"log/slog"
	"strings"
)

type requestIDKey struct{}

// WithRequestID stores the request id; every log line written with this ctx carries it.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestID returns the id stored by WithRequestID, or "".
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// New returns the process logger. main calls slog.SetDefault with it, once.
func New(w io.Writer, level slog.Level) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level, ReplaceAttr: redact})
	return slog.New(contextHandler{h})
}

// redact: sensitive keys never reach a log line.
func redact(_ []string, a slog.Attr) slog.Attr {
	switch strings.ToLower(a.Key) {
	case "authorization", "cookie", "set-cookie", "password", "token", "access_token",
		"refresh_token", "step_up_token", "secret", "api_key", "x-internal-api-key", "signature",
		"cvu", "cbu", "alias", "cuit", "dni", "document_number", "doc_number", "email", "totp_code", "verification_code":
		return slog.String(a.Key, "[REDACTED]")
	}
	return a
}

type contextHandler struct{ slog.Handler }

func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestID(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{h.Handler.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{h.Handler.WithGroup(name)}
}
