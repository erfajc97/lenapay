package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"

	openapi "github.com/erfajc97/lenapay/apps/api/api"
	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/httpx"
)

// Hand-mounted paths (in the contract, excluded from generation, AG2).
const (
	pathWebhook = "/webhooks/psp/transfers"
	pathEvents  = "/orgs/{orgId}/events"
)

// Deps are what the router needs; cmd/api builds them.
type Deps struct {
	Server             *Server
	Guard              *auth.Guard
	Webhook            http.Handler // payins.WebhookHandler
	Events             http.Handler // realtime.StreamHandler
	CORSOrigins        []string
	TrustedProxies     []string // CIDRs of the load balancer; empty = use the socket address
	RateLimitPerMinute int
	BodyLimitBytes     int64
	UploadLimitBytes   int64
	RequestTimeout     time.Duration
}

// NewRouter: request id → recover → access log → security headers → CORS → client IP →
// rate limit → body limit → GUARD (every route, by the contract's operationId); then a
// per-request timeout and, for the strict server, the spec validator (AG5).
func NewRouter(d Deps) (http.Handler, error) {
	spec, err := openapi.Load()
	if err != nil {
		return nil, err
	}
	operations, err := gorillamux.NewRouter(spec)
	if err != nil {
		return nil, fmt.Errorf("spec router: %w", err)
	}

	r := chi.NewRouter()
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, r, apperr.New(apperr.CodeNotFound, nil))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, r, apperr.New(apperr.CodeNotFound, nil))
	})
	r.Use(httpx.RequestID, httpx.Recover, httpx.AccessLog, httpx.SecurityHeaders,
		cors.Handler(cors.Options{
			AllowedOrigins: d.CORSOrigins,
			AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete},
			AllowedHeaders: httpx.CORSAllowedHeaders(),
			ExposedHeaders: []string{httpx.RequestIDHeader},
			MaxAge:         300,
		}),
		clientIP(d.TrustedProxies),
		httprate.LimitBy(d.RateLimitPerMinute, time.Minute, func(r *http.Request) (string, error) {
			return httprate.CanonicalizeIP(middleware.GetClientIP(r.Context())), nil
		}, httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			httpx.WriteError(w, r, apperr.New(apperr.CodeRateLimited, nil))
		})),
		httpx.BodyLimit(d.BodyLimitBytes, d.UploadLimitBytes),
		guard(operations, d.Guard), // authenticate BEFORE anything reads or validates the body
	)

	// Streams: no request timeout (the hub ends the stream with the client or at shutdown).
	r.Get(pathEvents, d.Events.ServeHTTP)

	r.Group(func(r chi.Router) {
		r.Use(httpx.Timeout(d.RequestTimeout))
		// Signed webhooks: the signature covers the raw body, so no validator decodes it first.
		r.Post(pathWebhook, d.Webhook.ServeHTTP)
	})

	r.Group(func(r chi.Router) {
		r.Use(httpx.Timeout(d.RequestTimeout))
		r.Use(nethttpmiddleware.OapiRequestValidatorWithOptions(spec, &nethttpmiddleware.Options{
			// Security is the guard's job (it knows the rules); the validator checks shapes.
			Options:              openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
			ErrorHandlerWithOpts: httpx.ValidationError,
		}))
		strict := api.NewStrictHandlerWithOptions(d.Server, nil, api.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  httpx.RequestErrorHandler,
			ResponseErrorHandlerFunc: httpx.ResponseErrorHandler,
		})
		api.HandlerWithOptions(strict, api.ChiServerOptions{
			BaseRouter: r,
			ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				httpx.WriteError(w, r, apperr.Wrap(apperr.CodeValidation, err, nil))
			},
		})
	})
	return r, nil
}

// clientIP states the trust model: X-Forwarded-For is read only from the proxies we run.
func clientIP(trusted []string) func(http.Handler) http.Handler {
	if len(trusted) == 0 {
		return middleware.ClientIPFromRemoteAddr
	}
	return middleware.ClientIPFromXFF(trusted...)
}

// guard finds the contract operation of the request and applies its access rule. A path the
// contract does not declare is a 404 before any handler runs.
func guard(operations routers.Router, g *auth.Guard) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions { // CORS preflight was answered above
				next.ServeHTTP(w, r)
				return
			}
			route, pathParams, err := operations.FindRoute(r)
			if err != nil {
				httpx.WriteError(w, r, apperr.Wrap(apperr.CodeNotFound, err, nil))
				return
			}
			ctx, err := g.Authorize(r.Context(), r, route.Operation.OperationID, pathParams)
			if err != nil {
				httpx.WriteError(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
