//go:build integration

package payerview_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/erfajc97/lenapay/apps/api/internal/accounts"
	"github.com/erfajc97/lenapay/apps/api/internal/app"
	"github.com/erfajc97/lenapay/apps/api/internal/erplink"
	erpfake "github.com/erfajc97/lenapay/apps/api/internal/erplink/provider/fake"
	"github.com/erfajc97/lenapay/apps/api/internal/identity"
	"github.com/erfajc97/lenapay/apps/api/internal/kyc"
	kycfake "github.com/erfajc97/lenapay/apps/api/internal/kyc/provider/fake"
	notifyfake "github.com/erfajc97/lenapay/apps/api/internal/notifications/provider/fake"
	"github.com/erfajc97/lenapay/apps/api/internal/orgs"
	"github.com/erfajc97/lenapay/apps/api/internal/payers"
	payoutfake "github.com/erfajc97/lenapay/apps/api/internal/payouts/provider/fake"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/config"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db/dbtest"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
	"github.com/erfajc97/lenapay/apps/api/internal/psp/provider/bind"
	pspfake "github.com/erfajc97/lenapay/apps/api/internal/psp/provider/fake"
)

var pg *dbtest.Postgres

func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }

const (
	fixturesDir = "../../fixtures/erp"
	password    = "Lenapay2026!"
	internalKey = "internal-key-0123456789"
)

type pinger struct{}

func (pinger) Ping(context.Context) error { return nil }

// env is the whole API (every real service, the real router with guard and validator) over a
// cloned database. The PSP is the in-process fake, or the BIND HTTP adapter against a running
// apps/bind-mock when BINDMOCK_URL is set (e.g. http://localhost:8090 with `docker compose up`).
type env struct {
	pool    *pgxpool.Pool
	svc     *app.Services
	h       http.Handler
	issuer  *auth.Issuer
	fakePSP *pspfake.Provider // nil in bind-mock mode
	mockURL string
}

func setup(t *testing.T) *env {
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
	e := &env{pool: pool, issuer: issuer, mockURL: os.Getenv("BINDMOCK_URL")}
	var provider psp.Provider
	if e.mockURL != "" {
		provider = bind.New(bind.Options{BaseURL: e.mockURL, Timeout: 10 * time.Second})
	} else {
		e.fakePSP = pspfake.New()
		provider = e.fakePSP
	}
	cfg := config.Config{
		Env: config.EnvDevelopment, RefreshTTL: time.Hour, DevFixedEmailCode: "123456", PersonalModeEnabled: true,
		BusinessTimeZone: "America/Argentina/Buenos_Aires", CVUShareBaseURL: "https://lenapay.app/pagar",
	}
	e.svc = app.NewServices(app.Platform{Pool: pool, Clock: clk, Jobs: jobs, Issuer: issuer, Config: cfg}, app.Ports{
		PSP: provider, KYC: kycfake.NewProvider(nil), KYCDocuments: kycfake.NewDocumentStore(),
		ERP: erpfake.NewDirectory(fixturesDir), Sender: notifyfake.NewSender(false),
		Documents: payoutfake.NewDiskDocumentStore(t.TempDir()),
	})
	ctx := t.Context()
	if err := e.svc.Ledger.EnsureSystemAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Fees.EnsureDefaultPlan(ctx); err != nil {
		t.Fatal(err)
	}
	e.h, err = app.NewRouter(app.Deps{
		Server:  app.NewServer(e.svc, issuer, pinger{}),
		Guard:   auth.NewGuard(app.AccessRules(), issuer, auth.NewPGDirectory(pool), internalKey),
		Webhook: http.NotFoundHandler(), Events: http.NotFoundHandler(),
		CORSOrigins: []string{"http://localhost:8081"}, RateLimitPerMinute: 100000,
		BodyLimitBytes: 1 << 20, UploadLimitBytes: 11 << 20, RequestTimeout: 20 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// user is a signed-up, verified, logged-in user.
type user struct {
	id      uuid.UUID
	session uuid.UUID
	token   string
}

func (e *env) user(t *testing.T, email string) user {
	t.Helper()
	ctx := t.Context()
	id, err := e.svc.Identity.Signup(ctx, identity.SignupInput{Email: email, Password: config.Secret(password)})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Identity.VerifyEmail(ctx, email, "123456"); err != nil {
		t.Fatal(err)
	}
	tokens, err := e.svc.Identity.Login(ctx, identity.LoginInput{Email: email, Password: config.Secret(password)})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := e.issuer.VerifyAccess(tokens.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	return user{id: id, session: claims.SessionID, token: tokens.AccessToken}
}

func (e *env) stepUp(t *testing.T, u user) string {
	t.Helper()
	token, _, err := e.issuer.IssueStepUp(u.id, u.session)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func pdf(kind string) kyc.DocumentUpload {
	return kyc.DocumentUpload{Kind: kind, FileName: kind + ".pdf", Content: strings.NewReader("%PDF-1.7 test " + kind)}
}

// personDraft is a user with a personal org whose KYC case carries the DNI but is NOT submitted.
func (e *env) personDraft(t *testing.T, email, dni string) (user, uuid.UUID) {
	t.Helper()
	ctx := t.Context()
	u := e.user(t, email)
	o, err := e.svc.Orgs.Create(ctx, auth.Principal{UserID: u.id}, orgs.CreateInput{Kind: orgs.KindPersonal, LegalName: "Persona " + dni})
	if err != nil {
		t.Fatal(err)
	}
	p := auth.Principal{UserID: u.id, OrgID: o.ID, Role: auth.RoleAdmin}
	if _, err := e.svc.KYC.Update(ctx, p, kyc.UpdateInput{Person: &kyc.PersonData{
		FullName: "Persona " + dni, DNI: dni, BirthDate: time.Date(1990, 1, 2, 0, 0, 0, 0, time.UTC),
		Address: kyc.Address{Street: "Gorriti", Number: "4500", City: "CABA", Province: "CABA", PostalCode: "C1414", Country: "AR"},
	}}); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{kyc.DocDNIFront, kyc.DocDNIBack, kyc.DocSelfie} {
		if _, err := e.svc.KYC.UploadDocument(ctx, p, pdf(k)); err != nil {
			t.Fatal(err)
		}
	}
	return u, o.ID
}

// person is a user whose personal org passed KYC (the fake vendor approves at submit, which
// runs the approval hooks), with its personal CVUs issued.
func (e *env) person(t *testing.T, email, dni string) (user, uuid.UUID) {
	t.Helper()
	u, orgID := e.personDraft(t, email, dni)
	c, err := e.svc.KYC.Submit(t.Context(), auth.Principal{UserID: u.id, OrgID: orgID, Role: auth.RoleAdmin})
	if err != nil || c.Status != kyc.StatusApproved {
		t.Fatalf("person kyc = %s, %v", c.Status, err)
	}
	if err := e.svc.Payers.CreatePersonalCVUs(t.Context(), orgID); err != nil {
		t.Fatal(err)
	}
	return u, orgID
}

// company is an approved company org with one project account.
type company struct {
	owner   user
	admin   auth.Principal
	project uuid.UUID
}

func (e *env) company(t *testing.T, email, name, cuit string) company {
	t.Helper()
	ctx := t.Context()
	u := e.user(t, email)
	o, err := e.svc.Orgs.Create(ctx, auth.Principal{UserID: u.id}, orgs.CreateInput{Kind: orgs.KindCompany, LegalName: name, CUIT: &cuit})
	if err != nil {
		t.Fatal(err)
	}
	p := auth.Principal{UserID: u.id, SessionID: u.session, OrgID: o.ID, Role: auth.RoleAdmin}
	rep := "20111222"
	if _, err := e.svc.KYC.Update(ctx, p, kyc.UpdateInput{Company: &kyc.CompanyData{
		CUIT: cuit, LegalName: name,
		LegalAddress:    kyc.Address{Street: "Av. del Libertador", Number: "5800", City: "CABA", Province: "CABA", PostalCode: "C1428", Country: "AR"},
		Representatives: []kyc.Representative{{FullName: "Diego Morales", DNI: rep, Position: "Presidente"}},
	}}); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{kyc.DocBylaws, kyc.DocAFIPConstancy, kyc.DocDNIFront, kyc.DocDNIBack, kyc.DocSelfie} {
		d := pdf(k)
		if k != kyc.DocBylaws && k != kyc.DocAFIPConstancy {
			d.RepresentativeDNI = &rep
		}
		if _, err := e.svc.KYC.UploadDocument(ctx, p, d); err != nil {
			t.Fatal(err)
		}
	}
	c, err := e.svc.KYC.Submit(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.KYC.Decide(ctx, c.ID, kyc.Decision{Approved: true}); err != nil {
		t.Fatal(err)
	}
	p.KYCStatus, p.StepUp = auth.KYCApproved, true
	acc, err := e.svc.Accounts.Create(ctx, p, accounts.CreateInput{Name: "Torre " + name[:4]})
	if err != nil {
		t.Fatal(err)
	}
	return company{owner: u, admin: p, project: acc.ID}
}

// payer creates a payer of the company's project with an ARS CVU and returns the CVU number.
func (e *env) payer(t *testing.T, c company, docType payers.DocType, doc string) (uuid.UUID, string) {
	t.Helper()
	ctx := t.Context()
	py, err := e.svc.Payers.Create(ctx, c.admin, payers.CreateInput{AccountID: c.project, Name: "Comprador " + doc, DocType: docType, DocNumber: doc})
	if err != nil {
		t.Fatal(err)
	}
	cvu, err := e.svc.Payers.CreateCVU(ctx, c.admin, py.ID, payers.CreateCVUInput{Currency: money.ARS, IdempotencyKey: "cvu-" + ids.New().String()})
	if err != nil {
		t.Fatal(err)
	}
	return py.ID, cvu.Number
}

// incoming delivers a transfer to one of our CVUs and credits it (what the webhook job does).
func (e *env) incoming(t *testing.T, cvu string, minor int64, originName string) {
	t.Helper()
	transferID := "trf-" + ids.New().String()
	if e.fakePSP != nil {
		e.fakePSP.AddIncoming(psp.IncomingTransfer{
			ID: transferID, CVU: cvu, Amount: money.New(minor, money.ARS), OriginName: originName, OccurredAt: time.Now().UTC(),
		})
	} else {
		transferID = e.mockIncoming(t, cvu, fmt.Sprintf("%d.%02d", minor/100, minor%100), originName)
	}
	if err := e.svc.Payins.ProcessTransfer(t.Context(), uuid.Nil, transferID); err != nil {
		t.Fatal(err)
	}
}

func (e *env) mockIncoming(t *testing.T, cvu, amount, originName string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"cvu": cvu, "amount": amount, "originName": originName})
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, e.mockURL+"/_mock/incoming-transfer", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Transfer struct {
			ID string `json:"id"`
		} `json:"transfer"`
	}
	raw, _ := io.ReadAll(res.Body)
	if err := json.Unmarshal(raw, &out); err != nil || out.Transfer.ID == "" {
		t.Fatalf("bind-mock incoming %d %s: %v", res.StatusCode, raw, err)
	}
	return out.Transfer.ID
}

// personalCVU is the person's personal ARS CVU (issued by CreatePersonalCVUs).
func (e *env) personalCVU(t *testing.T, orgID uuid.UUID) string {
	t.Helper()
	cvu, err := e.svc.Payers.PersonalCVU(t.Context(), orgID, money.ARS)
	if err != nil {
		t.Fatal(err)
	}
	return cvu.Number
}

func (e *env) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(t.Context(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// linkERP links the company to the fixture ERP org that has its CUIT and imports the snapshot.
func (e *env) linkERP(t *testing.T, c company, erpUser string) erplink.Link {
	t.Helper()
	ctx := t.Context()
	if err := e.svc.ERPLink.LoadIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	l, err := e.svc.ERPLink.Link(ctx, c.admin, erplink.Credentials{Username: erpUser, Password: config.Secret(erpfake.DevPassword)})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.ERPLink.Import(ctx, c.admin.OrgID); err != nil {
		t.Fatal(err)
	}
	return l
}

type call struct {
	method, path, token string
	body                any
	headers             map[string]string
}

func (e *env) do(t *testing.T, c call) (int, map[string]any) {
	t.Helper()
	var body io.Reader = http.NoBody
	if c.body != nil {
		raw, _ := json.Marshal(c.body)
		body = bytes.NewReader(raw)
	}
	r := httptest.NewRequest(c.method, c.path, body)
	if c.body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		r.Header.Set("Authorization", "Bearer "+c.token)
	}
	for k, v := range c.headers {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, r)
	var out map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s %s: decode %q: %v", c.method, c.path, rec.Body, err)
		}
	}
	return rec.Code, out
}

func (e *env) get(t *testing.T, token, path string) (int, map[string]any) {
	t.Helper()
	return e.do(t, call{method: http.MethodGet, path: path, token: token})
}

func errCode(body map[string]any) string {
	if e, ok := body["error"].(map[string]any); ok {
		s, _ := e["code"].(string)
		return s
	}
	return ""
}

func wantError(t *testing.T, status int, body map[string]any, wantStatus int, code string) {
	t.Helper()
	if status != wantStatus || errCode(body) != code {
		t.Fatalf("status %d code %q, want %d %q (body %v)", status, errCode(body), wantStatus, code, body)
	}
}

// keys asserts the exact key set of a JSON object (FT20: nothing more, nothing less).
func keys(t *testing.T, what string, v any, want ...string) {
	t.Helper()
	obj, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%s is %T, not an object", what, v)
	}
	got := make([]string, 0, len(obj))
	for k := range obj {
		got = append(got, k)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("%s keys = %v, want %v", what, got, want)
	}
}

func items(t *testing.T, body map[string]any) []map[string]any {
	t.Helper()
	raw, ok := body["items"].([]any)
	if !ok {
		t.Fatalf("no items in %v", body)
	}
	out := make([]map[string]any, 0, len(raw))
	for _, it := range raw {
		out = append(out, it.(map[string]any))
	}
	return out
}

func companyNamed(t *testing.T, list []map[string]any, name string) map[string]any {
	t.Helper()
	for _, c := range list {
		if c["companyName"] == name {
			return c
		}
	}
	t.Fatalf("company %q not in %v", name, list)
	return nil
}

func companiesPath(linkID any, rest string) string {
	return fmt.Sprintf("/me/payments/companies/%v%s", linkID, rest)
}
