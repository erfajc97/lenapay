package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"

	"lenapay/apps/api/internal/platform/apperr"
	applog "lenapay/apps/api/internal/platform/log"
)

// RequestIDHeader is accepted from clients when safe and always echoed back (A15).
const RequestIDHeader = "X-Request-ID"

var safeRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

// RequestID accepts a safe incoming X-Request-ID or makes one, returns it in the response
// header and stores it in the context for logs, errors and enqueued jobs.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if !safeRequestID.MatchString(id) {
			id = uuid.NewString()
		}
		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(applog.WithRequestID(r.Context(), id)))
	})
}

// Recover turns a panic into a logged 500 envelope. Panics are bugs, never control flow.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer recoverPanic(w, r)
		next.ServeHTTP(w, r)
	})
}

func recoverPanic(w http.ResponseWriter, r *http.Request) {
	rec := recover()
	if rec == nil {
		return
	}
	if rec == http.ErrAbortHandler { //nolint:errorlint // the stdlib compares this sentinel the same way
		panic(rec) // net/http uses ErrAbortHandler to abort a response: let it through
	}
	slog.ErrorContext(r.Context(), "panic", "panic", fmt.Sprint(rec), "stack", string(debug.Stack()))
	WriteError(w, r, errors.New("panic recovered"))
}

// AccessLog writes one line per request: method, path, status, duration. No query strings,
// headers or bodies (they may carry tokens or personal data).
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		slog.InfoContext(r.Context(), "http",
			"method", r.Method, "path", r.URL.Path, "status", sw.status,
			"duration_ms", time.Since(start).Milliseconds())
	})
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status, w.wroteHeader = code, true
	}
	w.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController reach Flush / SetWriteDeadline (SSE).
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// BodyLimit caps every request body (A12): jsonMax for everything, uploadMax for multipart
// uploads (KYC documents). A bigger body fails with request.too_large.
func BodyLimit(jsonMax, uploadMax int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			limit := jsonMax
			if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				limit = uploadMax
			}
			if r.ContentLength > limit {
				WriteError(w, r, apperr.New(apperr.CodeBodyTooLarge, nil))
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}

// Timeout gives every request a deadline; handlers pass ctx down, so the database and
// providers stop with it, and WriteError answers request.timeout.
func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SecurityHeaders: an API answers JSON only, is never framed and never cached (A5, A12).
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		h.Set("Cache-Control", "private, no-store")
		next.ServeHTTP(w, r)
	})
}

// CORSAllowedHeaders lists every custom request header the app sends (A12). The last three are
// what the web build's EventSource polyfill (react-native-sse) sends on the SSE stream.
func CORSAllowedHeaders() []string {
	return []string{
		"Authorization", "Content-Type", "Idempotency-Key", "X-Step-Up-Token", RequestIDHeader, "X-App-Version",
		"Cache-Control", "X-Requested-With", "Last-Event-ID",
	}
}
