//go:build integration

package app_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"io"
	"maps"
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

	"lenapay/apps/api/internal/accounts"
	"lenapay/apps/api/internal/app"
	"lenapay/apps/api/internal/erplink"
	erpfake "lenapay/apps/api/internal/erplink/provider/fake"
	"lenapay/apps/api/internal/identity"
	"lenapay/apps/api/internal/kyc"
	kycfake "lenapay/apps/api/internal/kyc/provider/fake"
	notifyfake "lenapay/apps/api/internal/notifications/provider/fake"
	"lenapay/apps/api/internal/orgs"
	"lenapay/apps/api/internal/payers"
	payoutfake "lenapay/apps/api/internal/payouts/provider/fake"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/clock"
	"lenapay/apps/api/internal/platform/config"
	"lenapay/apps/api/internal/platform/db/dbtest"
	"lenapay/apps/api/internal/platform/ids"
	"lenapay/apps/api/internal/platform/money"
	"lenapay/apps/api/internal/psp"
	pspfake "lenapay/apps/api/internal/psp/provider/fake"
)

var pg *dbtest.Postgres

func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }

const roundPassword = "Lenapay2026!"

// stack is the whole API (every real service, the real router with guard and validator) over a
// cloned database, with in-process fakes for the ports.
type stack struct {
	pool   *pgxpool.Pool
	svc    *app.Services
	h      http.Handler
	issuer *auth.Issuer
	psp    *pspfake.Provider
}

func newStack(t *testing.T) *stack {
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
	s := &stack{pool: pool, issuer: issuer, psp: pspfake.New()}
	cfg := config.Config{
		Env: config.EnvDevelopment, RefreshTTL: time.Hour, DevFixedEmailCode: "123456", PersonalModeEnabled: true,
		BusinessTimeZone: "America/Argentina/Buenos_Aires", CVUShareBaseURL: "https://lenapay.app/pagar",
	}
	s.svc = app.NewServices(app.Platform{Pool: pool, Clock: clk, Jobs: jobs, Issuer: issuer, Config: cfg}, app.Ports{
		PSP: s.psp, KYC: kycfake.NewProvider(nil), KYCDocuments: kycfake.NewDocumentStore(),
		ERP: erpfake.NewDirectory("../../fixtures/erp"), Sender: notifyfake.NewSender(false),
		Documents: payoutfake.NewDiskDocumentStore(t.TempDir()),
	})
	ctx := t.Context()
	if err := s.svc.Ledger.EnsureSystemAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.svc.Fees.EnsureDefaultPlan(ctx); err != nil {
		t.Fatal(err)
	}
	s.h, err = app.NewRouter(app.Deps{
		Server:  app.NewServer(s.svc, issuer, pinger{}),
		Guard:   auth.NewGuard(app.AccessRules(), issuer, auth.NewPGDirectory(pool), "internal-key-0123456789"),
		Webhook: http.NotFoundHandler(), Events: http.NotFoundHandler(),
		CORSOrigins: []string{"http://localhost:8081"}, RateLimitPerMinute: 100000,
		BodyLimitBytes: 1 << 20, UploadLimitBytes: 11 << 20, RequestTimeout: 20 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

type member struct {
	id      uuid.UUID
	session uuid.UUID
	token   string
}

// signup registers and verifies a user through the API (fullName optional) and logs in.
func (s *stack) signup(t *testing.T, email string, fullName *string) member {
	t.Helper()
	body := map[string]any{"email": email, "password": roundPassword}
	if fullName != nil {
		body["fullName"] = *fullName
	}
	s.must(t, req{method: http.MethodPost, path: "/auth/signup", body: body}, http.StatusCreated)
	s.must(t, req{method: http.MethodPost, path: "/auth/verify-email", body: map[string]any{"email": email, "code": "123456"}}, http.StatusNoContent)
	return s.login(t, email)
}

func (s *stack) login(t *testing.T, email string) member {
	t.Helper()
	tokens, err := s.svc.Identity.Login(t.Context(), identity.LoginInput{Email: email, Password: config.Secret(roundPassword)})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := s.issuer.VerifyAccess(tokens.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := claims.UserID()
	return member{id: id, session: claims.SessionID, token: tokens.AccessToken}
}

func (s *stack) stepUp(t *testing.T, m member) string {
	t.Helper()
	token, _, err := s.issuer.IssueStepUp(m.id, m.session)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func pdf(kind string) kyc.DocumentUpload {
	return kyc.DocumentUpload{Kind: kind, FileName: kind + ".pdf", Content: strings.NewReader("%PDF-1.7 test " + kind)}
}

// company is a KYC-approved company created by owner, with one project account.
func (s *stack) company(t *testing.T, owner member, name, cuit string) (orgID, project uuid.UUID) {
	t.Helper()
	ctx := t.Context()
	o, err := s.svc.Orgs.Create(ctx, auth.Principal{UserID: owner.id}, orgs.CreateInput{Kind: orgs.KindCompany, LegalName: name, CUIT: &cuit})
	if err != nil {
		t.Fatal(err)
	}
	p := auth.Principal{UserID: owner.id, SessionID: owner.session, OrgID: o.ID, Role: auth.RoleOwner}
	rep := "20111222"
	if _, err := s.svc.KYC.Update(ctx, p, kyc.UpdateInput{Company: &kyc.CompanyData{
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
		if _, err := s.svc.KYC.UploadDocument(ctx, p, d); err != nil {
			t.Fatal(err)
		}
	}
	c, err := s.svc.KYC.Submit(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.svc.KYC.Decide(ctx, c.ID, kyc.Decision{Approved: true}); err != nil {
		t.Fatal(err)
	}
	p.KYCStatus, p.StepUp = auth.KYCApproved, true
	acc, err := s.svc.Accounts.Create(ctx, p, accounts.CreateInput{Name: "Torre " + name[:4]})
	if err != nil {
		t.Fatal(err)
	}
	return o.ID, acc.ID
}

func ownerOf(m member, orgID uuid.UUID) auth.Principal {
	return auth.Principal{UserID: m.id, SessionID: m.session, OrgID: orgID, Role: auth.RoleOwner, KYCStatus: auth.KYCApproved, StepUp: true}
}

// fund credits the project through a payer's CVU, like a real payin.
func (s *stack) fund(t *testing.T, owner auth.Principal, project uuid.UUID, minor int64) {
	t.Helper()
	ctx := t.Context()
	py, err := s.svc.Payers.Create(ctx, owner, payers.CreateInput{AccountID: project, Name: "Comprador Uno", DocType: payers.DocDNI, DocNumber: "28111222"})
	if err != nil {
		t.Fatal(err)
	}
	cvu, err := s.svc.Payers.CreateCVU(ctx, owner, py.ID, payers.CreateCVUInput{Currency: money.ARS, IdempotencyKey: "cvu-" + ids.New().String()})
	if err != nil {
		t.Fatal(err)
	}
	transferID := "trf-" + ids.New().String()
	s.psp.AddIncoming(psp.IncomingTransfer{ID: transferID, CVU: cvu.Number, Amount: money.New(minor, money.ARS), OriginName: "Comprador Uno", OccurredAt: time.Now().UTC()})
	if err := s.svc.Payins.ProcessTransfer(ctx, uuid.Nil, transferID); err != nil {
		t.Fatal(err)
	}
}

func (s *stack) addMember(t *testing.T, owner auth.Principal, email string, role auth.Role) {
	t.Helper()
	if _, err := s.svc.Orgs.AddMember(t.Context(), owner, orgs.MemberInput{Email: email, Role: role}); err != nil {
		t.Fatal(err)
	}
}

type req struct {
	method, path, token, stepUp, idem string
	body                              any
}

func (s *stack) do(t *testing.T, r req) (int, map[string]any) {
	t.Helper()
	var body io.Reader = http.NoBody
	if r.body != nil {
		raw, _ := json.Marshal(r.body)
		body = bytes.NewReader(raw)
	}
	hr := httptest.NewRequest(r.method, r.path, body)
	if r.body != nil {
		hr.Header.Set("Content-Type", "application/json")
	}
	if r.token != "" {
		hr.Header.Set("Authorization", "Bearer "+r.token)
	}
	if r.stepUp != "" {
		hr.Header.Set(auth.HeaderStepUp, r.stepUp)
	}
	if r.idem != "" {
		hr.Header.Set("Idempotency-Key", r.idem)
	}
	rec := httptest.NewRecorder()
	s.h.ServeHTTP(rec, hr)
	var out map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s %s: decode %q: %v", r.method, r.path, rec.Body, err)
		}
	}
	return rec.Code, out
}

// must asserts the status and, when keys are given, the EXACT key set of the body (FT20).
func (s *stack) must(t *testing.T, r req, status int, keys ...string) map[string]any {
	t.Helper()
	code, out := s.do(t, r)
	if code != status {
		t.Fatalf("%s %s = %d %v, want %d", r.method, r.path, code, out, status)
	}
	if len(keys) > 0 {
		exactKeys(t, r.method+" "+r.path, out, keys...)
	}
	return out
}

// fail asserts an error status and code (FT3).
func (s *stack) fail(t *testing.T, r req, status int, code string) {
	t.Helper()
	got, out := s.do(t, r)
	errBody, _ := out["error"].(map[string]any)
	if got != status || errBody["code"] != code {
		t.Fatalf("%s %s = %d %v, want %d %s", r.method, r.path, got, out, status, code)
	}
}

func exactKeys(t *testing.T, where string, obj map[string]any, keys ...string) {
	t.Helper()
	got := slices.Sorted(maps.Keys(obj))
	want := slices.Sorted(slices.Values(keys))
	if !slices.Equal(got, want) {
		t.Fatalf("%s: keys %v, want %v", where, got, want)
	}
}

func ptr(s string) *string { return &s }

func (s *stack) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := s.pool.QueryRow(t.Context(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Owner decisions over HTTP: the creator is owner; only the owner (step-up) changes the approval
// policy — an admin trying is the attack — and transfers ownership; a payout strictly below the
// threshold is approved at creation, one at the threshold waits.
func TestOwnerPolicyAndTransferOverHTTP(t *testing.T) {
	t.Parallel()
	s := newStack(t)
	ana := s.signup(t, "ana@sur.test", ptr("Ana Pérez"))
	orgID, project := s.company(t, ana, "Constructora Sur SA", "30711111110")
	org := "/orgs/" + orgID.String()
	owner := ownerOf(ana, orgID)
	bruno := s.signup(t, "bruno@sur.test", nil)
	carla := s.signup(t, "carla@sur.test", nil)
	stranger := s.signup(t, "zoe@otra.test", nil)
	s.addMember(t, owner, "bruno@sur.test", auth.RoleAdmin)
	s.addMember(t, owner, "carla@sur.test", auth.RoleOperator)

	me := s.must(t, req{method: http.MethodGet, path: "/me", token: ana.token}, http.StatusOK, "user", "memberships", "preferences")
	user := me["user"].(map[string]any)
	exactKeys(t, "user", user, "id", "email", "fullName", "emailVerified", "mfaEnabled", "createdAt")
	if user["fullName"] != "Ana Pérez" {
		t.Fatalf("user %v", user)
	}
	roles := map[string]any{}
	for _, m := range me["memberships"].([]any) {
		mm := m.(map[string]any)
		roles[mm["orgId"].(string)] = mm["role"]
	}
	if roles[orgID.String()] != "owner" {
		t.Fatalf("memberships %v", me["memberships"])
	}
	members := s.must(t, req{method: http.MethodGet, path: org + "/members", token: carla.token}, http.StatusOK, "items")
	first := members["items"].([]any)[0].(map[string]any)
	exactKeys(t, "member", first, "userId", "email", "fullName", "role", "joinedAt")
	if first["fullName"] != "Ana Pérez" || first["role"] != "owner" {
		t.Fatalf("first member %v", first)
	}

	policyPath := org + "/approval-policy"
	policy := s.must(t, req{method: http.MethodGet, path: policyPath, token: carla.token}, http.StatusOK,
		"mode", "thresholdMinor", "allowOwnerSelfApproval", "ownerSelfApprovalEffective", "updatedBy", "updatedAt")
	exactKeys(t, "thresholdMinor", policy["thresholdMinor"].(map[string]any), "ARS", "USD")
	if policy["mode"] != "always" || policy["allowOwnerSelfApproval"] != true || policy["ownerSelfApprovalEffective"] != false {
		t.Fatalf("default policy %v (bruno is an admin: self-approval not effective)", policy)
	}
	threshold := map[string]any{"mode": "threshold", "thresholdMinor": map[string]any{"ARS": 50_000_000}, "allowOwnerSelfApproval": true}
	s.fail(t, req{method: http.MethodPut, path: policyPath, token: bruno.token, stepUp: s.stepUp(t, bruno), body: threshold}, http.StatusForbidden, "auth.forbidden")
	s.fail(t, req{method: http.MethodPut, path: policyPath, token: ana.token, body: threshold}, http.StatusForbidden, "auth.step_up_required")
	s.fail(t, req{method: http.MethodPut, path: policyPath, token: ana.token, stepUp: "not-a-token", body: threshold}, http.StatusForbidden, "auth.step_up_required")
	s.fail(t, req{
		method: http.MethodPut, path: policyPath, token: ana.token, stepUp: s.stepUp(t, ana),
		body: map[string]any{"mode": "threshold", "allowOwnerSelfApproval": true},
	}, http.StatusUnprocessableEntity, "validation.failed")
	s.fail(t, req{
		method: http.MethodPut, path: policyPath, token: ana.token, stepUp: s.stepUp(t, ana),
		body: map[string]any{"mode": "threshold", "thresholdMinor": map[string]any{"ARS": 0}, "allowOwnerSelfApproval": true},
	}, http.StatusUnprocessableEntity, "validation.failed")
	if n := s.count(t, "SELECT count(*) FROM approval_policies WHERE org_id = $1", orgID); n != 0 {
		t.Fatal("a refused policy change was stored")
	}
	updated := s.must(t, req{method: http.MethodPut, path: policyPath, token: ana.token, stepUp: s.stepUp(t, ana), body: threshold}, http.StatusOK,
		"mode", "thresholdMinor", "allowOwnerSelfApproval", "ownerSelfApprovalEffective", "updatedBy", "updatedAt")
	if updated["mode"] != "threshold" || updated["thresholdMinor"].(map[string]any)["ARS"] != float64(50_000_000) ||
		updated["thresholdMinor"].(map[string]any)["USD"] != nil || updated["updatedBy"] != ana.id.String() {
		t.Fatalf("updated policy %v", updated)
	}

	s.fund(t, owner, project, 200_000_000)
	pay := func(minor int64) map[string]any {
		return s.must(t, req{
			method: http.MethodPost, path: org + "/payouts", token: carla.token, stepUp: s.stepUp(t, carla), idem: "idem-" + ids.New().String(),
			body: map[string]any{"accountId": project.String(), "amount": map[string]any{"minor": minor, "currency": "ARS"}, "to": "0170099220000067797370", "concept": "Certificado 3"},
		}, http.StatusCreated)
	}
	if below := pay(49_999_999); below["status"] != "approved" && below["status"] != "sending" {
		t.Fatalf("below the threshold = %v", below["status"])
	}
	if at := pay(50_000_000); at["status"] != "pending_approval" {
		t.Fatalf("at the threshold = %v", at["status"])
	}

	transfer := org + "/ownership-transfer"
	s.fail(t, req{method: http.MethodPost, path: transfer, token: bruno.token, stepUp: s.stepUp(t, bruno), body: map[string]any{"userId": bruno.id}}, http.StatusForbidden, "auth.forbidden")
	s.fail(t, req{method: http.MethodPost, path: transfer, token: ana.token, stepUp: s.stepUp(t, ana), body: map[string]any{"userId": stranger.id}}, http.StatusNotFound, "member.not_found")
	s.fail(t, req{method: http.MethodPost, path: transfer, token: ana.token, stepUp: s.stepUp(t, ana), body: map[string]any{"userId": ana.id}}, http.StatusUnprocessableEntity, "validation.failed")
	s.fail(t, req{method: http.MethodPost, path: transfer, token: stranger.token, stepUp: s.stepUp(t, stranger), body: map[string]any{"userId": stranger.id}}, http.StatusNotFound, "org.not_found")
	moved := s.must(t, req{method: http.MethodPost, path: transfer, token: ana.token, stepUp: s.stepUp(t, ana), body: map[string]any{"userId": bruno.id}},
		http.StatusOK, "previousOwner", "owner")
	if moved["previousOwner"].(map[string]any)["role"] != "admin" || moved["owner"].(map[string]any)["role"] != "owner" {
		t.Fatalf("transfer %v", moved)
	}
	// The old owner is an admin now: the new owner may remove her; nobody removes the owner.
	s.fail(t, req{method: http.MethodPut, path: policyPath, token: ana.token, stepUp: s.stepUp(t, ana), body: threshold}, http.StatusForbidden, "auth.forbidden")
	s.fail(t, req{method: http.MethodDelete, path: org + "/members/" + bruno.id.String(), token: bruno.token}, http.StatusConflict, "org.last_admin")
	s.must(t, req{method: http.MethodDelete, path: org + "/members/" + ana.id.String(), token: bruno.token}, http.StatusNoContent)
}

// Preferences and profile over HTTP: absent keeps, null clears, the merged state is validated
// (an account needs its org, the org must be the caller's, the account a live one of it).
func TestPreferencesAndProfileOverHTTP(t *testing.T) {
	t.Parallel()
	s := newStack(t)
	ana := s.signup(t, "ana@sur.test", nil)
	orgID, project := s.company(t, ana, "Constructora Sur SA", "30711111110")
	beto := s.signup(t, "beto@norte.test", nil)
	otherOrg, otherProject := s.company(t, beto, "Desarrollos Norte SA", "30722222220")

	exactKeys(t, "prefs", s.must(t, req{method: http.MethodGet, path: "/me/preferences", token: ana.token}, http.StatusOK),
		"defaultOrgId", "defaultAccountId", "locale", "theme", "updatedAt")
	patch := func(body map[string]any) req {
		return req{method: http.MethodPatch, path: "/me/preferences", token: ana.token, body: body}
	}
	s.fail(t, patch(map[string]any{"defaultAccountId": project}), http.StatusUnprocessableEntity, "validation.failed")
	s.fail(t, patch(map[string]any{"defaultOrgId": otherOrg}), http.StatusNotFound, "org.not_found")
	s.fail(t, patch(map[string]any{"defaultOrgId": orgID, "defaultAccountId": otherProject}), http.StatusNotFound, "account.not_found")
	s.fail(t, patch(map[string]any{"locale": "fr-FR"}), http.StatusUnprocessableEntity, "validation.failed")
	s.fail(t, patch(map[string]any{"color": "red"}), http.StatusUnprocessableEntity, "validation.failed")

	set := s.must(t, patch(map[string]any{"defaultOrgId": orgID, "defaultAccountId": project, "locale": "en-US", "theme": "dark"}), http.StatusOK)
	if set["defaultOrgId"] != orgID.String() || set["defaultAccountId"] != project.String() || set["locale"] != "en-US" || set["theme"] != "dark" || set["updatedAt"] == nil {
		t.Fatalf("preferences %v", set)
	}
	kept := s.must(t, patch(map[string]any{"theme": nil}), http.StatusOK)
	if kept["theme"] != nil || kept["locale"] != "en-US" || kept["defaultAccountId"] != project.String() {
		t.Fatalf("partial update %v", kept)
	}
	me := s.must(t, req{method: http.MethodGet, path: "/me", token: ana.token}, http.StatusOK, "user", "memberships", "preferences")
	if me["preferences"].(map[string]any)["defaultOrgId"] != orgID.String() {
		t.Fatalf("/me preferences %v", me["preferences"])
	}
	// Clearing the org with its account set is refused (merged state); clearing both works.
	s.fail(t, patch(map[string]any{"defaultOrgId": nil}), http.StatusUnprocessableEntity, "validation.failed")
	archived, err := s.svc.Accounts.Create(t.Context(), ownerOf(ana, orgID), accounts.CreateInput{Name: "Obra cerrada"})
	if err != nil {
		t.Fatal(err)
	}
	yes := true
	if _, err := s.svc.Accounts.Update(t.Context(), ownerOf(ana, orgID), archived.ID, accounts.UpdateInput{Archived: &yes}); err != nil {
		t.Fatal(err)
	}
	s.fail(t, patch(map[string]any{"defaultAccountId": archived.ID}), http.StatusConflict, "account.archived")

	// Removed from the org, the default reads as unset (never pointing at a foreign org).
	s.addMember(t, ownerOf(beto, otherOrg), "ana@sur.test", auth.RoleViewer)
	s.must(t, patch(map[string]any{"defaultOrgId": otherOrg, "defaultAccountId": nil}), http.StatusOK)
	s.must(t, req{method: http.MethodDelete, path: "/orgs/" + otherOrg.String() + "/members/" + ana.id.String(), token: beto.token}, http.StatusNoContent)
	if gone := s.must(t, req{method: http.MethodGet, path: "/me/preferences", token: ana.token}, http.StatusOK); gone["defaultOrgId"] != nil || gone["locale"] != "en-US" {
		t.Fatalf("after removal %v", gone)
	}

	// Profile: trimmed, null clears, blank refused at both doors.
	named := s.must(t, req{method: http.MethodPatch, path: "/me", token: ana.token, body: map[string]any{"fullName": "  Ana Pérez  "}}, http.StatusOK, "user", "memberships", "preferences")
	if named["user"].(map[string]any)["fullName"] != "Ana Pérez" {
		t.Fatalf("named %v", named["user"])
	}
	s.fail(t, req{method: http.MethodPatch, path: "/me", token: ana.token, body: map[string]any{"fullName": "   "}}, http.StatusUnprocessableEntity, "validation.failed")
	s.fail(t, req{method: http.MethodPatch, path: "/me", token: ana.token, body: map[string]any{"fullName": strings.Repeat("a", 121)}}, http.StatusUnprocessableEntity, "validation.failed")
	s.must(t, req{method: http.MethodPatch, path: "/me", token: ana.token, body: map[string]any{"fullName": strings.Repeat("a", 120)}}, http.StatusOK)
	cleared := s.must(t, req{method: http.MethodPatch, path: "/me", token: ana.token, body: map[string]any{"fullName": nil}}, http.StatusOK)
	if cleared["user"].(map[string]any)["fullName"] != nil {
		t.Fatalf("cleared %v", cleared["user"])
	}
	s.fail(t, req{method: http.MethodPost, path: "/auth/signup", body: map[string]any{"email": "x@y.test", "password": roundPassword, "fullName": "  "}},
		http.StatusUnprocessableEntity, "validation.failed")
}

// The person KYC approval names the user when they have no name, and never overwrites theirs.
func TestPersonKYCFillsAnEmptyFullName(t *testing.T) {
	t.Parallel()
	s := newStack(t)
	for _, tt := range []struct {
		email  string
		signup *string
		want   string
	}{
		{"lucia@persona.test", nil, "Lucía Fernández"},
		{"mora@persona.test", ptr("Mora"), "Mora"},
	} {
		u := s.signup(t, tt.email, tt.signup)
		list, err := s.svc.Orgs.List(t.Context(), auth.Principal{UserID: u.id})
		if err != nil || len(list) != 1 || list[0].Kind != orgs.KindPersonal || list[0].Role != auth.RoleOwner {
			t.Fatalf("personal org %+v %v", list, err)
		}
		p := auth.Principal{UserID: u.id, OrgID: list[0].ID, Role: auth.RoleOwner}
		if _, err := s.svc.KYC.Update(t.Context(), p, kyc.UpdateInput{Person: &kyc.PersonData{
			FullName: "Lucía Fernández", DNI: "30123456", BirthDate: time.Date(1990, 1, 2, 0, 0, 0, 0, time.UTC),
			Address: kyc.Address{Street: "Gorriti", Number: "4500", City: "CABA", Province: "CABA", PostalCode: "C1414", Country: "AR"},
		}}); err != nil {
			t.Fatal(err)
		}
		for _, k := range []string{kyc.DocDNIFront, kyc.DocDNIBack, kyc.DocSelfie} {
			if _, err := s.svc.KYC.UploadDocument(t.Context(), p, pdf(k)); err != nil {
				t.Fatal(err)
			}
		}
		if c, err := s.svc.KYC.Submit(t.Context(), p); err != nil || c.Status != kyc.StatusApproved {
			t.Fatalf("kyc %v %v", c.Status, err)
		}
		me := s.must(t, req{method: http.MethodGet, path: "/me", token: u.token}, http.StatusOK)
		if got := me["user"].(map[string]any)["fullName"]; got != tt.want {
			t.Fatalf("%s: fullName %v, want %s", tt.email, got, tt.want)
		}
	}
}

// ERP users import end to end: the link's import invites the ERP users with the role their
// Lebane permissions map to (members skipped, no payments access skipped), GET erp-link reports
// it, and the invited people become members as soon as they verify their email (new users) or
// log in (users who already existed).
func TestERPUsersImportOverHTTP(t *testing.T) {
	t.Parallel()
	s := newStack(t)
	ana := s.signup(t, "ana@sur.test", nil)
	orgID, _ := s.company(t, ana, "Desarrollos Del Sur SA", "30712345678") // erp-org-001 in the fixtures
	owner := ownerOf(ana, orgID)
	s.signup(t, "cuentas@desarrollosdelsur.com.ar", nil)
	s.addMember(t, owner, "cuentas@desarrollosdelsur.com.ar", auth.RoleOperator)
	s.signup(t, "contable@desarrollosdelsur.com.ar", nil) // registered before the import

	if err := s.svc.ERPLink.LoadIdentityIndex(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.svc.ERPLink.Link(t.Context(), owner, erplink.Credentials{Username: "admin@desarrollosdelsur.com.ar", Password: config.Secret(erpfake.DevPassword)}); err != nil {
		t.Fatal(err)
	}
	link := s.must(t, req{method: http.MethodGet, path: "/orgs/" + orgID.String() + "/erp-link", token: ana.token}, http.StatusOK,
		"id", "erpOrgId", "displayName", "linkedBy", "linkedAt", "importStatus", "importedAt", "importSummary")
	if link["importSummary"] != nil {
		t.Fatalf("summary before the import %v", link["importSummary"])
	}
	if err := s.svc.ERPLink.Import(t.Context(), orgID); err != nil {
		t.Fatal(err)
	}
	link = s.must(t, req{method: http.MethodGet, path: "/orgs/" + orgID.String() + "/erp-link", token: ana.token}, http.StatusOK)
	sum := link["importSummary"].(map[string]any)
	exactKeys(t, "importSummary", sum, "projects", "buyers", "skippedBuyers", "usersInvited", "usersAlreadyMembers", "usersWithoutPaymentsAccess", "invitedUsers")
	if sum["usersInvited"] != float64(3) || sum["usersAlreadyMembers"] != float64(1) || sum["usersWithoutPaymentsAccess"] != float64(1) {
		t.Fatalf("summary %v", sum)
	}
	invited := map[string]any{}
	for _, u := range sum["invitedUsers"].([]any) {
		uu := u.(map[string]any)
		invited[uu["email"].(string)] = uu["role"]
	}
	want := map[string]any{"admin@desarrollosdelsur.com.ar": "admin", "tesoreria@desarrollosdelsur.com.ar": "operator", "contable@desarrollosdelsur.com.ar": "viewer"}
	if !maps.Equal(invited, want) {
		t.Fatalf("invited %v", invited)
	}

	roleIn := func(m member) any {
		orgsList := s.must(t, req{method: http.MethodGet, path: "/orgs", token: m.token}, http.StatusOK)
		for _, o := range orgsList["items"].([]any) {
			if oo := o.(map[string]any); oo["id"] == orgID.String() {
				return oo["role"]
			}
		}
		return nil
	}
	if r := roleIn(s.signup(t, "tesoreria@desarrollosdelsur.com.ar", ptr("Carolina Méndez"))); r != "operator" {
		t.Fatalf("new user after verification: role %v", r)
	}
	if r := roleIn(s.login(t, "contable@desarrollosdelsur.com.ar")); r != "viewer" {
		t.Fatalf("existing user after login: role %v", r)
	}
	if n := s.count(t, "SELECT count(*) FROM invites WHERE org_id = $1 AND source = 'erp_import' AND accepted_at IS NULL", orgID); n != 1 {
		t.Fatalf("pending erp invites %d (admin@ never signed up)", n)
	}
}
