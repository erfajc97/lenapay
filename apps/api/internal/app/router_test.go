package app_test

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"lenapay/apps/api/internal/accounts"
	"lenapay/apps/api/internal/app"
	"lenapay/apps/api/internal/erplink"
	"lenapay/apps/api/internal/identity"
	"lenapay/apps/api/internal/kyc"
	"lenapay/apps/api/internal/movements"
	"lenapay/apps/api/internal/notifications"
	"lenapay/apps/api/internal/orgs"
	"lenapay/apps/api/internal/payers"
	"lenapay/apps/api/internal/payins"
	"lenapay/apps/api/internal/payouts"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/clock"
	"lenapay/apps/api/internal/platform/httpx"
	"lenapay/apps/api/internal/platform/ids"
	"lenapay/apps/api/internal/realtime"
	"lenapay/apps/api/internal/reconciliation"
)

type pinger struct{ err error }

// handlerReached stands in for the accounts service: the router cases only prove a request
// got through the middleware to the handler, so the service answers 501 (no database here).
type handlerReached struct{ accounts.Service }

func (handlerReached) Create(context.Context, auth.Principal, accounts.CreateInput) (accounts.Account, error) {
	return accounts.Account{}, apperr.NotImplemented()
}

func (handlerReached) Transfer(context.Context, auth.Principal, accounts.TransferInput) (accounts.Transfer, error) {
	return accounts.Transfer{}, apperr.NotImplemented()
}

func (p pinger) Ping(context.Context) error { return p.err }

// directory: one active session, member of org as admin with approved KYC.
type directory struct {
	session, user, org uuid.UUID
}

func (d directory) ActiveSession(_ context.Context, userID, sessionID uuid.UUID) error {
	if userID != d.user || sessionID != d.session {
		return auth.ErrNotFound
	}
	return nil
}

func (d directory) Membership(_ context.Context, orgID, userID uuid.UUID) (auth.Membership, error) {
	if orgID != d.org || userID != d.user {
		return auth.Membership{}, auth.ErrNotFound
	}
	return auth.Membership{Role: auth.RoleAdmin, KYCStatus: auth.KYCApproved}, nil
}

type fixture struct {
	h           http.Handler
	token       string
	stepUp      string
	org         uuid.UUID
	internalKey string
}

func newFixture(t *testing.T, health error) fixture {
	t.Helper()
	issuer, err := auth.NewIssuer(auth.IssuerConfig{
		Issuer: "http://lenapay.test", Seed: make([]byte, ed25519.SeedSize), Clock: clock.Real{},
		AccessTTL: time.Minute, StepUpTTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	dir := directory{session: ids.New(), user: ids.New(), org: ids.New()}
	token, _, _ := issuer.IssueAccess(dir.user, dir.session)
	stepUp, _, _ := issuer.IssueStepUp(dir.user, dir.session)
	const internalKey = "internal-key-0123456789"

	server := &app.Server{
		IdentityHandler:       identity.NewHandler(identity.NewService(identity.Deps{}), issuer.JWKS),
		OrgsHandler:           orgs.NewHandler(orgs.NewService(orgs.Deps{})),
		KYCHandler:            kyc.NewHandler(kyc.NewService(kyc.Deps{})),
		AccountsHandler:       accounts.NewHandler(handlerReached{}),
		PayersHandler:         payers.NewHandler(payers.NewService(payers.Deps{})),
		PayinsHandler:         payins.NewHandler(payins.NewService(payins.Deps{})),
		PayoutsHandler:        payouts.NewHandler(payouts.NewService(payouts.Deps{})),
		MovementsHandler:      movements.NewHandler(movements.NewService(movements.Deps{})),
		ReconciliationHandler: reconciliation.NewHandler(reconciliation.NewService(reconciliation.Deps{})),
		ERPLinkHandler:        erplink.NewHandler(erplink.NewService(erplink.Deps{})),
		NotificationsHandler:  notifications.NewHandler(notifications.NewService(notifications.Deps{})),
		Health:                pinger{err: health},
	}
	h, err := app.NewRouter(app.Deps{
		Server:             server,
		Guard:              auth.NewGuard(app.AccessRules(), issuer, dir, internalKey),
		Webhook:            payins.NewWebhookHandler(payins.NewService(payins.Deps{Clock: clock.Real{}})),
		Events:             realtime.NewStreamHandler(nil),
		CORSOrigins:        []string{"http://localhost:8081"},
		RateLimitPerMinute: 1000,
		BodyLimitBytes:     1024,
		UploadLimitBytes:   4096,
		RequestTimeout:     5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	return fixture{h: h, token: token, stepUp: stepUp, org: dir.org, internalKey: internalKey}
}

type request struct {
	method, path, body string
	headers            map[string]string
}

func (f fixture) do(t *testing.T, req request) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(req.method, req.path, strings.NewReader(req.body))
	if req.body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	for k, v := range req.headers {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, r)
	var body map[string]any
	if rec.Body.Len() > 0 && strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode %q: %v", rec.Body.String(), err)
		}
	}
	return rec, body
}

func errorCode(body map[string]any) string {
	e, _ := body["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

func TestHealth(t *testing.T) {
	t.Parallel()
	rec, body := newFixture(t, nil).do(t, request{method: http.MethodGet, path: "/healthz"})
	if rec.Code != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("status %d body %v", rec.Code, body)
	}
	rec, body = newFixture(t, errors.New("db down")).do(t, request{method: http.MethodGet, path: "/healthz"})
	if rec.Code != http.StatusServiceUnavailable || errorCode(body) != string(apperr.CodeProviderUnavailable) {
		t.Fatalf("status %d body %v", rec.Code, body)
	}
}

// The HTTP contract of every error: status, catalog code, and the same request id in the
// header and the envelope (A15). Each failing case differs from a passing request in ONE way (FT4).
func TestRouterErrors(t *testing.T) {
	t.Parallel()
	f := newFixture(t, nil)
	bearer := map[string]string{"Authorization": "Bearer " + f.token}
	orgPath := "/orgs/" + f.org.String()
	validAccount := `{"name":"Torre Norte"}`
	transfer := `{"fromAccountId":"` + ids.New().String() + `","toAccountId":"` + ids.New().String() + `","amount":{"minor":1000,"currency":"ARS"}}`
	moneyHeaders := map[string]string{"Authorization": "Bearer " + f.token, "Idempotency-Key": "client-key-0000000001", "X-Step-Up-Token": f.stepUp}

	tests := []struct {
		name       string
		req        request
		wantStatus int
		wantCode   apperr.Code
	}{
		{"valid request reaches the stub", request{http.MethodPost, orgPath + "/accounts", validAccount, bearer}, http.StatusNotImplemented, apperr.CodeNotImplemented},
		{"route not in the contract", request{http.MethodGet, "/nope", "", bearer}, http.StatusNotFound, apperr.CodeNotFound},
		{"no token", request{http.MethodPost, orgPath + "/accounts", validAccount, nil}, http.StatusUnauthorized, apperr.CodeUnauthenticated},
		{"bad token", request{http.MethodPost, orgPath + "/accounts", validAccount, map[string]string{"Authorization": "Bearer bad"}}, http.StatusUnauthorized, apperr.CodeUnauthenticated},
		{"foreign org is not found", request{http.MethodPost, "/orgs/" + ids.New().String() + "/accounts", validAccount, bearer}, http.StatusNotFound, apperr.CodeOrgNotFound},
		{"unknown field is rejected", request{http.MethodPost, orgPath + "/accounts", `{"name":"Torre Norte","orgId":"x"}`, bearer}, http.StatusUnprocessableEntity, apperr.CodeValidation},
		{"over-long string is rejected", request{http.MethodPost, orgPath + "/accounts", `{"name":"` + strings.Repeat("x", 81) + `"}`, bearer}, http.StatusUnprocessableEntity, apperr.CodeValidation},
		{"body over the limit", request{http.MethodPost, orgPath + "/accounts", `{"name":"` + strings.Repeat("x", 2048) + `"}`, bearer}, http.StatusRequestEntityTooLarge, apperr.CodeBodyTooLarge},
		{"money route without step-up", request{http.MethodPost, orgPath + "/internal-transfers", transfer, bearer}, http.StatusForbidden, apperr.CodeStepUpRequired},
		{"money route reaches the stub", request{http.MethodPost, orgPath + "/internal-transfers", transfer, moneyHeaders}, http.StatusNotImplemented, apperr.CodeNotImplemented},
		{"amount as a float is rejected", request{http.MethodPost, orgPath + "/internal-transfers", strings.Replace(transfer, "1000", "10.5", 1), moneyHeaders}, http.StatusUnprocessableEntity, apperr.CodeValidation},
		{"internal route without key", request{http.MethodPost, "/internal/reconciliation/run", `{"currency":"ARS","day":"2026-11-07"}`, nil}, http.StatusUnauthorized, apperr.CodeUnauthenticated},
		// With the key the guard lets the request through to the validator (the run itself is lane D's integration test).
		{"internal route with key", request{http.MethodPost, "/internal/reconciliation/run", `{"currency":"ARS","day":"2026-11-07","extra":1}`, map[string]string{"X-Internal-Api-Key": f.internalKey}}, http.StatusUnprocessableEntity, apperr.CodeValidation},
		{"unsigned webhook reaches its handler and is refused", request{http.MethodPost, "/webhooks/psp/transfers", `{}`, nil}, http.StatusUnauthorized, apperr.CodeWebhookSignature},
		{"event stream is guarded", request{http.MethodGet, orgPath + "/events", "", nil}, http.StatusUnauthorized, apperr.CodeUnauthenticated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec, body := f.do(t, tt.req)
			if rec.Code != tt.wantStatus || errorCode(body) != string(tt.wantCode) {
				t.Fatalf("status %d code %q, want %d %s (body %s)", rec.Code, errorCode(body), tt.wantStatus, tt.wantCode, rec.Body)
			}
			e := body["error"].(map[string]any)
			if id := rec.Header().Get(httpx.RequestIDHeader); id == "" || e["requestId"] != id {
				t.Fatalf("request id header %q and envelope %v must match", id, e["requestId"])
			}
		})
	}
}

// FT20: the error envelope has exactly the contract's keys.
func TestErrorEnvelopeKeys(t *testing.T) {
	t.Parallel()
	_, body := newFixture(t, nil).do(t, request{method: http.MethodGet, path: "/nope"})
	if len(body) != 1 {
		t.Fatalf("top-level keys %v", body)
	}
	var keys []string
	for k := range body["error"].(map[string]any) {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if strings.Join(keys, ",") != "code,message,requestId" {
		t.Fatalf("envelope keys %v", keys)
	}
}

func TestJWKSIsPublic(t *testing.T) {
	t.Parallel()
	rec, body := newFixture(t, nil).do(t, request{method: http.MethodGet, path: "/.well-known/jwks.json"})
	keys, _ := body["keys"].([]any)
	if rec.Code != http.StatusOK || len(keys) != 1 {
		t.Fatalf("status %d body %v", rec.Code, body)
	}
	if k := keys[0].(map[string]any); k["kty"] != "OKP" || k["alg"] != "EdDSA" {
		t.Fatalf("key %v", k)
	}
}

func TestCORSPreflight(t *testing.T) {
	t.Parallel()
	f := newFixture(t, nil)
	r := httptest.NewRequest(http.MethodOptions, "/orgs", nil)
	r.Header.Set("Origin", "http://localhost:8081")
	r.Header.Set("Access-Control-Request-Method", http.MethodPost)
	r.Header.Set("Access-Control-Request-Headers", "Idempotency-Key, X-Step-Up-Token")
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, r)
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:8081" {
		t.Fatalf("preflight headers %v", rec.Header())
	}

	// The web SSE client (react-native-sse) sends these on GET /orgs/{orgId}/events.
	sse := httptest.NewRequest(http.MethodOptions, "/orgs/00000000-0000-0000-0000-000000000001/events", nil)
	sse.Header.Set("Origin", "http://localhost:8081")
	sse.Header.Set("Access-Control-Request-Method", http.MethodGet)
	sse.Header.Set("Access-Control-Request-Headers", "authorization, cache-control, x-requested-with, last-event-id")
	rec = httptest.NewRecorder()
	f.h.ServeHTTP(rec, sse)
	allowed := strings.ToLower(rec.Header().Get("Access-Control-Allow-Headers"))
	for _, h := range []string{"cache-control", "x-requested-with", "last-event-id"} {
		if !strings.Contains(allowed, h) {
			t.Fatalf("SSE preflight does not allow %s: %q", h, allowed)
		}
	}
	r.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	f.h.ServeHTTP(rec, r)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("foreign origin allowed")
	}
}
