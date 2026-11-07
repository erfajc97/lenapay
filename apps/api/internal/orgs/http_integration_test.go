//go:build integration

package orgs_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/erfajc97/lenapay/apps/api/internal/app"
	"github.com/erfajc97/lenapay/apps/api/internal/identity"
	"github.com/erfajc97/lenapay/apps/api/internal/kyc"
	kycfake "github.com/erfajc97/lenapay/apps/api/internal/kyc/provider/fake"
	"github.com/erfajc97/lenapay/apps/api/internal/orgs"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/audit"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/config"
)

const internalKey = "internal-key-0123456789"

type nopHook struct{}

func (nopHook) OnKYCApproved(context.Context, pgx.Tx, uuid.UUID, string) error { return nil }

type pinger struct{}

func (pinger) Ping(context.Context) error { return nil }

// api is the real router (guard, validator, strict server) over the lane A services and a
// real database; the other features are not mounted (their routes are never called here).
type api struct {
	h    http.Handler
	mail *mailbox
}

func newAPI(t *testing.T, personal bool) api {
	t.Helper()
	pool := pg.NewDatabase(t)
	clk := clock.Real{}
	issuer, err := auth.NewIssuer(auth.IssuerConfig{Issuer: "http://lenapay.test", Seed: make([]byte, ed25519.SeedSize), Clock: clk, AccessTTL: 15 * time.Minute, StepUpTTL: 5 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	jobs, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Env: config.EnvDevelopment, RefreshTTL: time.Hour, DevFixedEmailCode: "123456", PersonalModeEnabled: personal}
	box := &mailbox{tokens: map[string]string{}, codes: map[string]string{}}
	var orgSvc orgs.Service
	var kycSvc kyc.Service
	idSvc := identity.NewService(identity.Deps{
		Pool: pool, Clock: clk, Issuer: issuer, Codes: box, Config: cfg,
		Memberships: identity.MembershipsFunc(func(ctx context.Context, id uuid.UUID) ([]identity.MembershipSummary, error) {
			return orgSvc.MembershipsForUser(ctx, id)
		}),
	})
	orgSvc = orgs.NewService(orgs.Deps{
		Pool: pool, Clock: clk, Audit: audit.NewLogger(clk), Users: idSvc, Mailer: box, Config: cfg,
		KYC: orgs.KYCCasesFunc(func(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, kind string) error {
			return kycSvc.OpenCase(ctx, tx, orgID, kind)
		}),
	})
	kycSvc = kyc.NewService(kyc.Deps{
		Pool: pool, Clock: clk, Audit: audit.NewLogger(clk), Jobs: jobs, Provider: kycfake.NewProvider(nil),
		Documents: kycfake.NewDocumentStore(), Orgs: orgSvc, OnApprove: []kyc.ApprovalHook{nopHook{}},
	})
	h, err := app.NewRouter(app.Deps{
		Server: &app.Server{
			IdentityHandler: identity.NewHandler(idSvc, issuer.JWKS), OrgsHandler: orgs.NewHandler(orgSvc),
			KYCHandler: kyc.NewHandler(kycSvc), Health: pinger{},
		},
		Guard:   auth.NewGuard(app.AccessRules(), issuer, auth.NewPGDirectory(pool), internalKey),
		Webhook: http.NotFoundHandler(), Events: http.NotFoundHandler(),
		CORSOrigins: []string{"http://localhost:8081"}, RateLimitPerMinute: 10000,
		BodyLimitBytes: 1 << 20, UploadLimitBytes: 11 << 20, RequestTimeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	return api{h: h, mail: box}
}

type call struct {
	method, path string
	body         any
	token        string
	headers      map[string]string
	multipart    *bytes.Buffer
	contentType  string
}

func (a api) do(t *testing.T, c call) (int, map[string]any) {
	t.Helper()
	var body *bytes.Buffer
	switch {
	case c.multipart != nil:
		body = c.multipart
	case c.body != nil:
		b, _ := json.Marshal(c.body)
		body = bytes.NewBuffer(b)
	default:
		body = &bytes.Buffer{}
	}
	r := httptest.NewRequest(c.method, c.path, body)
	switch {
	case c.multipart != nil:
		r.Header.Set("Content-Type", c.contentType)
	case c.body != nil:
		r.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		r.Header.Set("Authorization", "Bearer "+c.token)
	}
	for k, v := range c.headers {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, r)
	var out map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s %s: decode %q: %v", c.method, c.path, rec.Body, err)
		}
	}
	return rec.Code, out
}

// must asserts the status and, for 2xx JSON, the EXACT response key set (A5, FT20).
func (a api) must(t *testing.T, c call, status int, keys ...string) map[string]any {
	t.Helper()
	code, out := a.do(t, c)
	if code != status {
		t.Fatalf("%s %s: status %d, want %d (%v)", c.method, c.path, code, status, out)
	}
	if keys != nil {
		got := make([]string, 0, len(out))
		for k := range out {
			got = append(got, k)
		}
		slices.Sort(got)
		slices.Sort(keys)
		if !slices.Equal(got, keys) {
			t.Fatalf("%s %s: keys %v, want %v", c.method, c.path, got, keys)
		}
	}
	return out
}

func (a api) fail(t *testing.T, c call, status int, code string) {
	t.Helper()
	got, out := a.do(t, c)
	e, _ := out["error"].(map[string]any)
	if got != status || e["code"] != code {
		t.Fatalf("%s %s: %d %v, want %d %s", c.method, c.path, got, out, status, code)
	}
}

// login signs up, verifies (DEV_FIXED_EMAIL_CODE) and logs in; it returns the token pair.
func (a api) login(t *testing.T, email string) map[string]any {
	t.Helper()
	cred := map[string]any{"email": email, "password": "una-clave-larga-123"}
	a.must(t, call{method: http.MethodPost, path: "/auth/signup", body: cred}, http.StatusCreated, "userId")
	a.must(t, call{method: http.MethodPost, path: "/auth/verify-email", body: map[string]any{"email": email, "code": "123456"}}, http.StatusNoContent)
	return a.must(t, call{method: http.MethodPost, path: "/auth/login", body: cred}, http.StatusOK,
		"accessToken", "accessTokenExpiresAt", "refreshToken", "refreshTokenExpiresAt", "tokenType")
}

func upload(t *testing.T, fields map[string]string, fileName string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	fw, err := w.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(content)
	_ = w.Close()
	return &buf, w.FormDataContentType()
}

// The lane A golden path through HTTP: signup → verify → login → device + step-up → org →
// invite → accept → KYC (data, documents, submit, ops decision) → /me → refresh reuse → logout.
func TestHTTPLaneAFlow(t *testing.T) {
	t.Parallel()
	a := newAPI(t, false)
	ana := a.login(t, "ana@example.com")
	tok := ana["accessToken"].(string)

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	dev := a.must(t, call{
		method: http.MethodPost, path: "/devices", token: tok,
		body: map[string]any{"publicKey": base64.RawURLEncoding.EncodeToString(pub), "name": "Pixel"},
	}, http.StatusCreated, "id", "name", "createdAt")
	ch := a.must(t, call{method: http.MethodPost, path: "/auth/step-up/challenge", token: tok, body: map[string]any{"deviceId": dev["id"]}},
		http.StatusCreated, "challengeId", "nonce", "expiresAt")
	nonce, _ := base64.RawURLEncoding.DecodeString(ch["nonce"].(string))
	a.must(t, call{method: http.MethodPost, path: "/auth/step-up/verify", token: tok, body: map[string]any{
		"challengeId": ch["challengeId"], "method": "device_signature", "deviceId": dev["id"],
		"signature": base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, nonce)),
	}}, http.StatusOK, "stepUpToken", "expiresAt")

	org := a.must(t, call{method: http.MethodPost, path: "/orgs", token: tok, body: map[string]any{"kind": "company", "legalName": "Constructora Sur"}},
		http.StatusCreated, "id", "kind", "legalName", "cuit", "kycStatus", "role", "createdAt")
	orgPath := "/orgs/" + org["id"].(string)
	a.must(t, call{method: http.MethodGet, path: orgPath, token: tok}, http.StatusOK, "id", "kind", "legalName", "cuit", "kycStatus", "role", "createdAt")
	a.must(t, call{method: http.MethodGet, path: "/orgs", token: tok}, http.StatusOK, "items")

	a.must(t, call{method: http.MethodPost, path: orgPath + "/invites", token: tok, body: map[string]any{"email": "bea@example.com", "role": "viewer"}},
		http.StatusCreated, "id", "email", "role", "expiresAt", "createdAt")
	bea := a.login(t, "bea@example.com")["accessToken"].(string)
	a.must(t, call{method: http.MethodPost, path: "/invites/" + a.mail.tokens["bea@example.com"] + "/accept", token: bea},
		http.StatusOK, "orgId", "orgName", "orgKind", "role", "kycStatus")
	members := a.must(t, call{method: http.MethodGet, path: orgPath + "/members", token: bea}, http.StatusOK, "items")
	if len(members["items"].([]any)) != 2 {
		t.Fatalf("members %v", members)
	}
	a.fail(t, call{method: http.MethodPut, path: orgPath + "/kyc", token: bea, body: map[string]any{}}, http.StatusForbidden, "auth.forbidden")
	carlos := a.login(t, "carlos@example.com")["accessToken"].(string)
	a.fail(t, call{method: http.MethodGet, path: orgPath, token: carlos}, http.StatusNotFound, "org.not_found")
	a.fail(t, call{method: http.MethodGet, path: orgPath + "/kyc", token: carlos}, http.StatusNotFound, "org.not_found")
	a.must(t, call{method: http.MethodGet, path: orgPath + "/kyc", token: bea}, http.StatusOK,
		"id", "orgId", "kind", "status", "documents", "decisionReason", "submittedAt", "decidedAt", "updatedAt")
	a.fail(t, call{method: http.MethodPost, path: orgPath + "/members", token: bea, body: map[string]any{"email": "carlos@example.com", "role": "viewer"}},
		http.StatusForbidden, "auth.forbidden")
	added := a.must(t, call{method: http.MethodPost, path: orgPath + "/members", token: tok, body: map[string]any{"email": "carlos@example.com", "role": "operator"}},
		http.StatusCreated, "userId", "email", "role", "joinedAt")
	carlosPath := orgPath + "/members/" + added["userId"].(string)
	a.fail(t, call{method: http.MethodPatch, path: carlosPath, token: bea, body: map[string]any{"role": "viewer"}}, http.StatusForbidden, "auth.forbidden")
	a.fail(t, call{method: http.MethodPatch, path: carlosPath, token: tok, body: map[string]any{"role": "owner"}}, http.StatusUnprocessableEntity, "validation.failed")
	changed := a.must(t, call{method: http.MethodPatch, path: carlosPath, token: tok, body: map[string]any{"role": "approver"}},
		http.StatusOK, "userId", "email", "role", "joinedAt")
	if changed["role"] != "approver" {
		t.Fatalf("role change %v", changed)
	}
	a.fail(t, call{method: http.MethodPatch, path: orgPath + "/members/" + me0(t, a, tok), token: tok, body: map[string]any{"role": "viewer"}},
		http.StatusConflict, "org.last_admin")
	a.must(t, call{method: http.MethodDelete, path: carlosPath, token: tok}, http.StatusNoContent)
	a.fail(t, call{method: http.MethodDelete, path: carlosPath, token: tok}, http.StatusNotFound, "member.not_found")
	a.must(t, call{method: http.MethodPost, path: orgPath + "/members", token: tok, body: map[string]any{"email": "carlos@example.com", "role": "operator"}},
		http.StatusCreated, "userId", "email", "role", "joinedAt")
	a.must(t, call{method: http.MethodPost, path: "/auth/resend-code", body: map[string]any{"email": "nobody@example.com"}}, http.StatusAccepted)

	kycCase := completeCompanyKYC(t, a, tok, orgPath)
	a.fail(t, call{method: http.MethodPost, path: "/internal/kyc/" + kycCase + "/decision", body: map[string]any{"decision": "approved"}},
		http.StatusUnauthorized, "auth.unauthenticated")
	decided := a.must(t, call{
		method: http.MethodPost, path: "/internal/kyc/" + kycCase + "/decision", body: map[string]any{"decision": "approved"},
		headers: map[string]string{auth.HeaderInternalKey: internalKey},
	}, http.StatusOK,
		"id", "orgId", "kind", "status", "company", "documents", "decisionReason", "submittedAt", "decidedAt", "updatedAt")
	if decided["status"] != "approved" {
		t.Fatalf("decision %v", decided)
	}
	me := a.must(t, call{method: http.MethodGet, path: "/me", token: tok}, http.StatusOK, "user", "memberships")
	if m := me["memberships"].([]any)[0].(map[string]any); m["kycStatus"] != "approved" || m["role"] != "admin" {
		t.Fatalf("membership %v", m)
	}

	next := a.must(t, call{method: http.MethodPost, path: "/auth/refresh", body: map[string]any{"refreshToken": ana["refreshToken"]}}, http.StatusOK,
		"accessToken", "accessTokenExpiresAt", "refreshToken", "refreshTokenExpiresAt", "tokenType")
	a.must(t, call{method: http.MethodGet, path: "/me", token: next["accessToken"].(string)}, http.StatusOK, "user", "memberships")
	a.fail(t, call{method: http.MethodPost, path: "/auth/refresh", body: map[string]any{"refreshToken": ana["refreshToken"]}},
		http.StatusUnauthorized, "auth.refresh_invalid")
	a.fail(t, call{method: http.MethodGet, path: "/me", token: next["accessToken"].(string)}, http.StatusUnauthorized, "auth.unauthenticated")

	a.must(t, call{method: http.MethodPost, path: "/auth/logout", token: bea}, http.StatusNoContent)
	a.fail(t, call{method: http.MethodGet, path: "/me", token: bea}, http.StatusUnauthorized, "auth.unauthenticated")
}

// me0 is the caller's user id (GET /me).
func me0(t *testing.T, a api, tok string) string {
	t.Helper()
	me := a.must(t, call{method: http.MethodGet, path: "/me", token: tok}, http.StatusOK, "user", "memberships")
	return me["user"].(map[string]any)["id"].(string)
}

// completeCompanyKYC fills, documents and submits the company case; it returns the case id.
func completeCompanyKYC(t *testing.T, a api, tok, orgPath string) string {
	t.Helper()
	address := map[string]any{"street": "Mitre", "number": "50", "city": "Rosario", "province": "Santa Fe", "postalCode": "2000"}
	data := map[string]any{"company": map[string]any{
		"cuit": "30712345678", "legalName": "Constructora Sur SA", "legalAddress": address,
		"representatives": []any{map[string]any{"fullName": "Carla Díaz", "dni": "20111222", "position": "Director"}},
	}}
	updated := a.must(t, call{method: http.MethodPut, path: orgPath + "/kyc", token: tok, body: data}, http.StatusOK,
		"id", "orgId", "kind", "status", "company", "documents", "decisionReason", "submittedAt", "decidedAt", "updatedAt")
	a.fail(t, call{method: http.MethodPost, path: orgPath + "/kyc/submit", token: tok}, http.StatusUnprocessableEntity, "kyc.incomplete")

	svg, svgType := upload(t, map[string]string{"kind": "bylaws"}, "bylaws.pdf", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`))
	a.fail(t, call{method: http.MethodPost, path: orgPath + "/kyc/documents", token: tok, multipart: svg, contentType: svgType},
		http.StatusUnprocessableEntity, "kyc.document_invalid")
	docs := []map[string]string{
		{"kind": "bylaws"},
		{"kind": "afip_constancy"},
		{"kind": "dni_front", "representativeDni": "20111222"},
		{"kind": "dni_back", "representativeDni": "20111222"},
		{"kind": "selfie", "representativeDni": "20111222"},
	}
	for _, fields := range docs {
		body, ct := upload(t, fields, fields["kind"]+".png", []byte("\x89PNG\r\n\x1a\n"+strings.Repeat("x", 64)))
		doc := a.must(t, call{method: http.MethodPost, path: orgPath + "/kyc/documents", token: tok, multipart: body, contentType: ct}, http.StatusCreated)
		if doc["contentType"] != "image/png" || doc["kind"] != fields["kind"] {
			t.Fatalf("document %v", doc)
		}
	}
	submitted := a.must(t, call{method: http.MethodPost, path: orgPath + "/kyc/submit", token: tok}, http.StatusOK)
	if submitted["status"] != "submitted" || len(submitted["documents"].([]any)) != 5 {
		t.Fatalf("submitted %v", submitted)
	}
	return updated["id"].(string)
}

// The contract caps reach the handlers before any service: each case differs from a valid
// body in ONE field (FT4).
func TestHTTPValidation(t *testing.T) {
	t.Parallel()
	a := newAPI(t, false)
	tests := []struct {
		name string
		body map[string]any
	}{
		{"password one char short", map[string]any{"email": "v@example.com", "password": "123456789"}},
		{"unknown field", map[string]any{"email": "v@example.com", "password": "una-clave-larga-123", "role": "admin"}},
		{"not an email", map[string]any{"email": "not-an-email", "password": "una-clave-larga-123"}},
	}
	for _, tt := range tests {
		a.fail(t, call{method: http.MethodPost, path: "/auth/signup", body: tt.body}, http.StatusUnprocessableEntity, "validation.failed")
	}
	a.must(t, call{method: http.MethodPost, path: "/auth/signup", body: map[string]any{"email": "v@example.com", "password": "1234567890"}},
		http.StatusCreated, "userId") // exactly 10 chars passes
	a.fail(t, call{method: http.MethodPost, path: "/auth/verify-email", body: map[string]any{"email": "v@example.com", "code": "12345"}},
		http.StatusUnprocessableEntity, "validation.failed")
	tok := a.login(t, "w@example.com")["accessToken"].(string)
	a.fail(t, call{method: http.MethodPost, path: "/devices", token: tok, body: map[string]any{"publicKey": strings.Repeat("!", 43), "name": "x"}},
		http.StatusUnprocessableEntity, "validation.failed")
	a.fail(t, call{method: http.MethodPost, path: "/orgs", token: tok, body: map[string]any{"kind": "personal", "legalName": "Wanda"}},
		http.StatusForbidden, "org.personal_mode_disabled")
	a.fail(t, call{method: http.MethodPost, path: "/auth/login", body: map[string]any{"email": "w@example.com", "password": "otra"}},
		http.StatusUnauthorized, "auth.invalid_credentials")
	a.fail(t, call{method: http.MethodGet, path: "/me"}, http.StatusUnauthorized, "auth.unauthenticated")
}

// The password fallback of step-up through HTTP (no device on the challenge).
func TestHTTPStepUpPassword(t *testing.T) {
	t.Parallel()
	a := newAPI(t, false)
	tok := a.login(t, "x@example.com")["accessToken"].(string)
	ch := a.must(t, call{method: http.MethodPost, path: "/auth/step-up/challenge", token: tok, body: map[string]any{}}, http.StatusCreated)
	a.fail(t, call{method: http.MethodPost, path: "/auth/step-up/verify", token: tok, body: map[string]any{
		"challengeId": ch["challengeId"], "method": "password", "password": "wrong-password",
	}}, http.StatusUnauthorized, "auth.invalid_credentials")
	ch = a.must(t, call{method: http.MethodPost, path: "/auth/step-up/challenge", token: tok, body: map[string]any{}}, http.StatusCreated)
	a.must(t, call{method: http.MethodPost, path: "/auth/step-up/verify", token: tok, body: map[string]any{
		"challengeId": ch["challengeId"], "method": "password", "password": "una-clave-larga-123",
	}}, http.StatusOK, "stepUpToken", "expiresAt")
	ch = a.must(t, call{method: http.MethodPost, path: "/auth/step-up/challenge", token: tok, body: map[string]any{}}, http.StatusCreated)
	a.fail(t, call{method: http.MethodPost, path: "/auth/step-up/verify", token: tok, body: map[string]any{
		"challengeId": ch["challengeId"], "method": "device_signature", "signature": "!!!",
	}}, http.StatusUnprocessableEntity, "validation.failed")
}

// R-KYC-2 through HTTP: a personal wallet is approved by the vendor at submit.
func TestHTTPPersonKYC(t *testing.T) {
	t.Parallel()
	a := newAPI(t, true)
	tok := a.login(t, "y@example.com")["accessToken"].(string)
	org := a.must(t, call{method: http.MethodPost, path: "/orgs", token: tok, body: map[string]any{"kind": "personal", "legalName": "Yamila Ruiz"}},
		http.StatusCreated)
	orgPath := "/orgs/" + org["id"].(string)
	person := map[string]any{
		"fullName": "Yamila Ruiz", "dni": "25111222", "birthDate": "1990-05-17",
		"address": map[string]any{"street": "Belgrano", "number": "10", "floor": "3B", "city": "Salta", "province": "Salta", "postalCode": "4400", "country": "AR"},
	}
	a.fail(t, call{method: http.MethodPut, path: orgPath + "/kyc", token: tok, body: map[string]any{"company": map[string]any{
		"cuit": "30712345678", "legalName": "Yamila SRL", "legalAddress": person["address"],
		"representatives": []any{map[string]any{"fullName": "Yamila Ruiz", "dni": "25111222", "position": "Socia"}},
	}}}, http.StatusUnprocessableEntity, "validation.failed")
	updated := a.must(t, call{method: http.MethodPut, path: orgPath + "/kyc", token: tok, body: map[string]any{"person": person}}, http.StatusOK)
	got, _ := json.Marshal(updated["person"])
	want, _ := json.Marshal(person)
	if string(got) != string(want) {
		t.Fatalf("person round trip %s, want %s", got, want)
	}
	for _, kind := range []string{"dni_front", "dni_back", "selfie"} {
		body, ct := upload(t, map[string]string{"kind": kind}, kind+".jpg", []byte("\xFF\xD8\xFF\xE0 jpeg"))
		a.must(t, call{method: http.MethodPost, path: orgPath + "/kyc/documents", token: tok, multipart: body, contentType: ct}, http.StatusCreated)
	}
	submitted := a.must(t, call{method: http.MethodPost, path: orgPath + "/kyc/submit", token: tok}, http.StatusOK)
	if submitted["status"] != "approved" || submitted["kind"] != "person" {
		t.Fatalf("submitted %v", submitted)
	}
}
