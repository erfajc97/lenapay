package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/jackc/pgx/v5/pgconn"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"

	"lenapay/apps/api/internal/platform/apperr"
)

func decodeEnvelope(t *testing.T, body io.Reader) EnvelopeBody {
	t.Helper()
	var env Envelope
	if err := json.NewDecoder(body).Decode(&env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	return env.Error
}

func TestClassify(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want apperr.Code
	}{
		{"domain passes through", fmt.Errorf("wrapped: %w", apperr.New(apperr.CodePayoutNotFound, nil)), apperr.CodePayoutNotFound},
		{"body too large", &http.MaxBytesError{Limit: 1}, apperr.CodeBodyTooLarge},
		{"deadline", fmt.Errorf("query: %w", context.DeadlineExceeded), apperr.CodeTimeout},
		{"unique violation", &pgconn.PgError{Code: "23505"}, apperr.CodeConflict},
		{"serialization failure", &pgconn.PgError{Code: "40001"}, apperr.CodeOperationInProgress},
		{"deadlock", &pgconn.PgError{Code: "40P01"}, apperr.CodeOperationInProgress},
		{"other pg error", &pgconn.PgError{Code: "23514"}, apperr.CodeInternal},
		{"unexpected", errors.New("boom"), apperr.CodeInternal},
	}
	for _, tt := range tests {
		if got := classify(tt.err).Code; got != tt.want {
			t.Errorf("%s: classify = %s, want %s", tt.name, got, tt.want)
		}
	}
}

// Internal errors never leak their text to the client.
func TestWriteErrorHidesInternals(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	WriteError(rec, httptest.NewRequest(http.MethodGet, "/", nil), errors.New("dial tcp 10.0.0.5:5432: secret-host"))
	body := decodeEnvelope(t, rec.Body)
	if rec.Code != http.StatusInternalServerError || body.Code != apperr.CodeInternal {
		t.Fatalf("status %d code %s", rec.Code, body.Code)
	}
	if strings.Contains(body.Message, "secret-host") || rec.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("message %q headers %v", body.Message, rec.Header())
	}
}

func TestBodyLimit(t *testing.T) {
	t.Parallel()
	h := BodyLimit(10, 20)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			WriteError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	cases := []struct {
		name        string
		size        int
		contentType string
		chunked     bool
		want        int
	}{
		{"json at limit", 10, "application/json", false, http.StatusNoContent},
		{"json one over", 11, "application/json", false, http.StatusRequestEntityTooLarge},
		{"json over without length", 11, "application/json", true, http.StatusRequestEntityTooLarge},
		{"upload at upload limit", 20, "multipart/form-data; boundary=x", false, http.StatusNoContent},
		{"upload one over", 21, "multipart/form-data; boundary=x", false, http.StatusRequestEntityTooLarge},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("a", c.size)))
			req.Header.Set("Content-Type", c.contentType)
			if c.chunked {
				req.ContentLength = -1
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != c.want {
				t.Fatalf("status %d, want %d", rec.Code, c.want)
			}
		})
	}
}

func TestRecover(t *testing.T) {
	t.Parallel()
	h := RequestID(Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") })))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body := decodeEnvelope(t, rec.Body)
	if rec.Code != http.StatusInternalServerError || body.Code != apperr.CodeInternal || body.RequestID != rec.Header().Get(RequestIDHeader) {
		t.Fatalf("status %d body %+v", rec.Code, body)
	}
}

func TestRequestIDAcceptsOnlySafeIDs(t *testing.T) {
	t.Parallel()
	h := RequestID(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	for sent, reused := range map[string]bool{"client-req-0001": true, "bad id\nnewline": false, "": false} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(RequestIDHeader, sent)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		got := rec.Header().Get(RequestIDHeader)
		if got == "" || (got == sent) != reused {
			t.Errorf("sent %q, got %q, reused want %v", sent, got, reused)
		}
	}
}

func TestTimeoutSetsADeadline(t *testing.T) {
	t.Parallel()
	var deadline time.Time
	Timeout(time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, _ = r.Context().Deadline()
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if deadline.IsZero() {
		t.Fatal("no deadline")
	}
}

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	SecurityHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Header().Get("Cache-Control") != "private, no-store" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("headers %v", rec.Header())
	}
}

func TestNewServerSetsEveryTimeout(t *testing.T) {
	t.Parallel()
	srv := NewServer(":0", http.NotFoundHandler(), slogDiscard())
	if srv.ReadHeaderTimeout == 0 || srv.ReadTimeout == 0 || srv.WriteTimeout == 0 || srv.IdleTimeout == 0 || srv.MaxHeaderBytes == 0 {
		t.Fatalf("server %+v", srv)
	}
}

func slogDiscard() *slog.Logger { return slog.New(slog.DiscardHandler) }

func TestValidationErrorMapping(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		err    error
		status int
		want   apperr.Code
		detail string
	}{
		{"unknown route", errors.New("no matching operation"), http.StatusNotFound, apperr.CodeNotFound, ""},
		{"schema error names the field", &openapi3.SchemaError{Reason: "maxLength"}, http.StatusBadRequest, apperr.CodeValidation, "field"},
		{"parameter error names the param", &openapi3filter.RequestError{Parameter: &openapi3.Parameter{Name: "limit"}, Err: errors.New("bad")}, http.StatusBadRequest, apperr.CodeValidation, "param"},
		{"oversized body", &http.MaxBytesError{Limit: 1}, http.StatusBadRequest, apperr.CodeBodyTooLarge, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			status := c.status
			if c.want == apperr.CodeNotFound {
				status = http.StatusNotFound
			}
			ValidationError(t.Context(), c.err, rec, req, nethttpmiddleware.ErrorHandlerOpts{StatusCode: status})
			body := decodeEnvelope(t, rec.Body)
			if body.Code != c.want {
				t.Fatalf("code %s, want %s", body.Code, c.want)
			}
			if c.detail != "" {
				if _, ok := body.Details[c.detail]; !ok {
					t.Fatalf("details %v lack %q", body.Details, c.detail)
				}
			}
		})
	}
}

func TestRequestErrorHandler(t *testing.T) {
	t.Parallel()
	for err, want := range map[error]apperr.Code{
		errors.New("invalid JSON"):    apperr.CodeValidation,
		&http.MaxBytesError{Limit: 1}: apperr.CodeBodyTooLarge,
	} {
		rec := httptest.NewRecorder()
		RequestErrorHandler(rec, httptest.NewRequest(http.MethodPost, "/", nil), err)
		if got := decodeEnvelope(t, rec.Body).Code; got != want {
			t.Errorf("%v: code %s, want %s", err, got, want)
		}
	}
	rec := httptest.NewRecorder()
	ResponseErrorHandler(rec, httptest.NewRequest(http.MethodGet, "/", nil), apperr.New(apperr.CodePayoutNotFound, nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
}
