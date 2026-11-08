// Package httpx is the HTTP edge: the ONE error writer (AG4), the middleware stack and the
// server with every timeout set (AG5).
package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/jackc/pgx/v5/pgconn"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"

	"lenapay/apps/api/internal/platform/apperr"
	applog "lenapay/apps/api/internal/platform/log"
)

// Envelope is the error body of every non-2xx answer: {"error":{code,message,details,requestId}}
// (contracts/openapi.yaml `Error`; the router test asserts the exact key set).
type Envelope struct {
	Error EnvelopeBody `json:"error"`
}

// EnvelopeBody is the inner object of Envelope.
type EnvelopeBody struct {
	Code      apperr.Code    `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	RequestID string         `json:"requestId"`
}

// Postgres SQLSTATEs the writer understands.
const (
	pgUniqueViolation      = "23505"
	pgSerializationFailure = "40001"
	pgDeadlockDetected     = "40P01"
)

// classify turns any error into a catalog *apperr.Error.
func classify(err error) *apperr.Error {
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return appErr
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return apperr.Wrap(apperr.CodeBodyTooLarge, err, nil)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return apperr.Wrap(apperr.CodeTimeout, err, nil)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgUniqueViolation:
			return apperr.Wrap(apperr.CodeConflict, err, nil)
		case pgSerializationFailure, pgDeadlockDetected:
			return apperr.Wrap(apperr.CodeOperationInProgress, err, nil)
		}
	}
	return apperr.Wrap(apperr.CodeInternal, err, nil)
}

// WriteError is the ONLY place that turns an error into an HTTP response (AG4). Server
// errors are logged once here with the request id; their text never reaches the client.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	ctx := r.Context()
	appErr := classify(err)
	status := appErr.Code.Status()
	if status >= http.StatusInternalServerError && appErr.Code != apperr.CodeNotImplemented {
		slog.ErrorContext(ctx, "request failed", "code", string(appErr.Code), "err", err.Error())
	}
	WriteJSON(w, status, Envelope{Error: EnvelopeBody{
		Code:      appErr.Code,
		Message:   appErr.Code.Message(),
		Details:   appErr.Details,
		RequestID: applog.RequestID(ctx),
	}})
}

// WriteJSON writes v with the status; private responses are never cached (A5).
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("write response", "err", err.Error())
	}
}

// RequestErrorHandler maps body/param decoding failures of the strict server.
func RequestErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		WriteError(w, r, err)
		return
	}
	WriteError(w, r, apperr.Wrap(apperr.CodeValidation, err, nil))
}

// ResponseErrorHandler maps every error a strict handler returns.
func ResponseErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	WriteError(w, r, err)
}

// ValidationError is the spec validator's error hook (nethttp-middleware ErrorHandlerWithOpts).
// Details name the failing field or parameter and never echo a value.
func ValidationError(_ context.Context, err error, w http.ResponseWriter, r *http.Request, opts nethttpmiddleware.ErrorHandlerOpts) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		WriteError(w, r, err)
		return
	}
	if opts.StatusCode == http.StatusNotFound || opts.StatusCode == http.StatusMethodNotAllowed {
		WriteError(w, r, apperr.Wrap(apperr.CodeNotFound, err, nil))
		return
	}
	details := map[string]any{}
	var schemaErr *openapi3.SchemaError
	if errors.As(err, &schemaErr) {
		details["field"] = "/" + strings.Join(schemaErr.JSONPointer(), "/")
	}
	var reqErr *openapi3filter.RequestError
	if errors.As(err, &reqErr) && reqErr.Parameter != nil {
		details["param"] = reqErr.Parameter.Name
	}
	WriteError(w, r, apperr.Wrap(apperr.CodeValidation, err, details))
}
