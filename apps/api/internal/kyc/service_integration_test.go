//go:build integration

package kyc_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/erfajc97/lenapay/apps/api/internal/identity"
	"github.com/erfajc97/lenapay/apps/api/internal/kyc"
	kycfake "github.com/erfajc97/lenapay/apps/api/internal/kyc/provider/fake"
	"github.com/erfajc97/lenapay/apps/api/internal/orgs"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/audit"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/config"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db/dbtest"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/upload"
)

var pg *dbtest.Postgres

func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }

// hook records approvals; fail makes it fail (to prove it runs in the approval transaction).
type hook struct {
	mu    sync.Mutex
	calls []string
	fail  error
}

func (h *hook) OnKYCApproved(_ context.Context, _ pgx.Tx, orgID uuid.UUID, orgKind string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.fail != nil {
		return h.fail
	}
	h.calls = append(h.calls, orgID.String()+":"+orgKind)
	return nil
}

type silent struct{}

func (silent) SendEmailCode(context.Context, string, string) error { return nil }
func (silent) SendInvite(context.Context, string, string, string) error {
	return nil
}

type env struct {
	pool     *pgxpool.Pool
	clock    *clock.Fake
	users    identity.Service
	orgs     orgs.Service
	kyc      kyc.Service
	provider *kycfake.Provider
	docs     *kycfake.DocumentStore
	hook     *hook
}

func newEnv(t *testing.T, autoApprove time.Duration) env {
	t.Helper()
	pool := pg.NewDatabase(t)
	clk := clock.NewFake(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	issuer, err := auth.NewIssuer(auth.IssuerConfig{Issuer: "http://lenapay.test", Seed: make([]byte, ed25519.SeedSize), Clock: clk, AccessTTL: time.Minute, StepUpTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	jobs, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{}) // insert-only, like cmd/api
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Env: config.EnvDevelopment, RefreshTTL: time.Hour, PersonalModeEnabled: true, DevFixedEmailCode: "123456"}
	e := env{pool: pool, clock: clk, provider: kycfake.NewProvider([]string{"11111111"}), docs: kycfake.NewDocumentStore(), hook: &hook{}}
	e.users = identity.NewService(identity.Deps{Pool: pool, Clock: clk, Issuer: issuer, Codes: silent{}, Config: cfg})
	e.orgs = orgs.NewService(orgs.Deps{
		Pool: pool, Clock: clk, Audit: audit.NewLogger(clk), Users: e.users, Mailer: silent{}, Config: cfg,
		KYC: orgs.KYCCasesFunc(func(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, kind string) error {
			return e.kyc.OpenCase(ctx, tx, orgID, kind)
		}),
	})
	e.kyc = kyc.NewService(kyc.Deps{
		Pool: pool, Clock: clk, Audit: audit.NewLogger(clk), Jobs: jobs, Provider: e.provider,
		Documents: e.docs, Orgs: e.orgs, OnApprove: []kyc.ApprovalHook{e.hook}, AutoApproveAfter: autoApprove,
	})
	return e
}

// org creates a user and an org of that kind; the principal acts as its admin.
func (e env) org(t *testing.T, kind orgs.Kind) auth.Principal {
	t.Helper()
	ctx := t.Context()
	email := ids.New().String() + "@example.com"
	userID, err := e.users.Signup(ctx, identity.SignupInput{Email: email, Password: "una-clave-larga-123"})
	if err != nil {
		t.Fatal(err)
	}
	p := auth.Principal{UserID: userID}
	o, err := e.orgs.Create(ctx, p, orgs.CreateInput{Kind: kind, LegalName: "Org " + string(kind)})
	if err != nil {
		t.Fatal(err)
	}
	p.OrgID, p.Role = o.ID, auth.RoleAdmin
	return p
}

func (e env) upload(t *testing.T, p auth.Principal, kind string, rep *string) kyc.Document {
	t.Helper()
	d, err := e.kyc.UploadDocument(t.Context(), p, kyc.DocumentUpload{
		Kind: kind, RepresentativeDNI: rep, FileName: kind + ".pdf", Content: strings.NewReader("%PDF-1.7 " + kind),
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func person(dni string) *kyc.PersonData {
	return &kyc.PersonData{
		FullName: "Ana Pérez", DNI: dni, BirthDate: time.Date(1990, 5, 17, 0, 0, 0, 0, time.UTC),
		Address: kyc.Address{Street: "Av. Corrientes", Number: "1234", City: "CABA", Province: "CABA", PostalCode: "C1043", Country: "AR"},
	}
}

func company(dni string) *kyc.CompanyData {
	return &kyc.CompanyData{
		CUIT: "30712345678", LegalName: "Constructora Sur SA",
		LegalAddress:    kyc.Address{Street: "Mitre", Number: "50", City: "Rosario", Province: "Santa Fe", PostalCode: "2000", Country: "AR"},
		Representatives: []kyc.Representative{{FullName: "Carla Díaz", DNI: dni, Position: "Director"}},
	}
}

func (e env) completePerson(t *testing.T, p auth.Principal, dni string) {
	t.Helper()
	if _, err := e.kyc.Update(t.Context(), p, kyc.UpdateInput{Person: person(dni)}); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{kyc.DocDNIFront, kyc.DocDNIBack, kyc.DocSelfie} {
		e.upload(t, p, k, nil)
	}
}

func (e env) completeCompany(t *testing.T, p auth.Principal) {
	t.Helper()
	dni := "20111222"
	if _, err := e.kyc.Update(t.Context(), p, kyc.UpdateInput{Company: company(dni)}); err != nil {
		t.Fatal(err)
	}
	e.upload(t, p, kyc.DocBylaws, nil)
	e.upload(t, p, kyc.DocAFIPConstancy, nil)
	for _, k := range []string{kyc.DocDNIFront, kyc.DocDNIBack, kyc.DocSelfie} {
		e.upload(t, p, k, &dni)
	}
}

func (e env) orgStatus(t *testing.T, orgID uuid.UUID) string {
	t.Helper()
	var s string
	if err := e.pool.QueryRow(t.Context(), "SELECT kyc_status FROM organizations WHERE id = $1", orgID).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func (e env) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(t.Context(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func wantCode(t *testing.T, err error, code apperr.Code) {
	t.Helper()
	if !apperr.IsCode(err, code) {
		t.Fatalf("err = %v, want %s", err, code)
	}
}

// R-KYC-2 + R-KYC-3: a person is decided by the vendor at submit; approval mirrors the org
// status and runs the approval hooks with the org kind.
func TestPersonApprovedByVendor(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 0)
	p := e.org(t, orgs.KindPersonal)
	e.completePerson(t, p, "25111222")

	c, err := e.kyc.Submit(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Status != kyc.StatusApproved || c.Kind != kyc.KindPerson || len(c.Documents) != 3 || c.Person.DNI != "25111222" || c.DecidedAt == nil {
		t.Fatalf("case %+v", c)
	}
	if got := e.orgStatus(t, p.OrgID); got != "approved" {
		t.Fatalf("org status %s", got)
	}
	if n := e.count(t, "SELECT count(*) FROM organizations WHERE id = $1 AND legal_name = 'Ana Pérez' AND doc_type = 'dni' AND doc_number = '25111222' AND cuit IS NULL", p.OrgID); n != 1 {
		t.Fatal("verified person identity not applied")
	}
	if len(e.hook.calls) != 1 || e.hook.calls[0] != p.OrgID.String()+":personal" {
		t.Fatalf("hook calls %v", e.hook.calls)
	}
	if n := e.count(t, "SELECT count(*) FROM audit_log WHERE action IN ('kyc.submit', 'kyc.approved') AND org_id = $1", p.OrgID); n != 2 {
		t.Fatalf("audit rows %d", n)
	}
	// The backstop review job was enqueued in the submit transaction; running it is a no-op.
	if n := e.count(t, "SELECT count(*) FROM river_job WHERE kind = 'kyc_review' AND args->>'case_id' = $1", c.ID.String()); n != 1 {
		t.Fatalf("review jobs %d", n)
	}
	if err := (&kyc.ReviewWorker{Service: e.kyc}).Work(t.Context(), &river.Job[kyc.ReviewArgs]{Args: kyc.ReviewArgs{CaseID: c.ID}}); err != nil {
		t.Fatal(err)
	}
	if len(e.hook.calls) != 1 {
		t.Fatal("a decided case must not be approved twice")
	}
}

// A rejected DNI rejects the case; fixing the data reopens it as draft and a new submission
// is checked again (a new vendor reference, not the cached rejection).
func TestPersonRejectedThenFixed(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 0)
	ctx := t.Context()
	p := e.org(t, orgs.KindPersonal)
	e.completePerson(t, p, "11111111")
	c, err := e.kyc.Submit(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Status != kyc.StatusRejected || c.DecisionReason == nil || *c.DecisionReason != kycfake.ReasonDNIRejected {
		t.Fatalf("case %+v", c)
	}
	if e.orgStatus(t, p.OrgID) != "rejected" || len(e.hook.calls) != 0 {
		t.Fatal("a rejection must not approve the org")
	}
	c, err = e.kyc.Update(ctx, p, kyc.UpdateInput{Person: person("25111222")})
	if err != nil || c.Status != kyc.StatusDraft || e.orgStatus(t, p.OrgID) != "draft" {
		t.Fatalf("reopen %+v %v", c, err)
	}
	e.clock.Advance(time.Minute)
	c, err = e.kyc.Submit(ctx, p)
	if err != nil || c.Status != kyc.StatusApproved || c.DecisionReason != nil {
		t.Fatalf("resubmit %+v %v", c, err)
	}
}

// Uncertain is not "rejected": a vendor outage leaves the case submitted and the review job
// decides later.
func TestVendorOutageLeavesCaseSubmitted(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 0)
	p := e.org(t, orgs.KindPersonal)
	e.completePerson(t, p, "25111222")
	e.provider.FailNext(1)
	c, err := e.kyc.Submit(t.Context(), p)
	if err != nil || c.Status != kyc.StatusSubmitted {
		t.Fatalf("submit during outage %+v %v", c, err)
	}
	if err := e.kyc.Review(t.Context(), c.ID); err != nil {
		t.Fatal(err)
	}
	c, _ = e.kyc.Get(t.Context(), p)
	if c.Status != kyc.StatusApproved {
		t.Fatalf("after review %s", c.Status)
	}
}

// R-KYC-1: a company waits for the ops decision; nothing changes after a final decision.
func TestCompanyDecidedByOps(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 0)
	ctx := t.Context()
	p := e.org(t, orgs.KindCompany)

	_, err := e.kyc.Submit(ctx, p)
	wantCode(t, err, apperr.CodeKYCIncomplete)
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || len(appErr.Details["missing"].([]string)) != 3 {
		t.Fatalf("missing details %v", err)
	}
	e.completeCompany(t, p)
	c, err := e.kyc.Submit(ctx, p)
	if err != nil || c.Status != kyc.StatusSubmitted || e.orgStatus(t, p.OrgID) != "submitted" {
		t.Fatalf("submit %+v %v", c, err)
	}
	if n := e.count(t, "SELECT count(*) FROM river_job WHERE kind = 'kyc_review'"); n != 0 {
		t.Fatalf("no auto-approval job without the dev delay, got %d", n)
	}
	if err := e.kyc.Review(ctx, c.ID); err != nil || len(e.hook.calls) != 0 {
		t.Fatalf("review must leave a company to ops: %v %v", err, e.hook.calls)
	}
	_, err = e.kyc.Submit(ctx, p)
	wantCode(t, err, apperr.CodeKYCInvalidState)

	c, err = e.kyc.Decide(ctx, c.ID, kyc.Decision{Approved: true})
	if err != nil || c.Status != kyc.StatusApproved || e.orgStatus(t, p.OrgID) != "approved" {
		t.Fatalf("decide %+v %v", c, err)
	}
	if len(e.hook.calls) != 1 || e.hook.calls[0] != p.OrgID.String()+":company" {
		t.Fatalf("hooks %v", e.hook.calls)
	}
	// The verified CUIT and legal name are now the org's (the ERP link looks it up by CUIT).
	if cuit, err := e.orgs.OrgCUIT(ctx, p.OrgID); err != nil || cuit != "30712345678" {
		t.Fatalf("org cuit %q %v", cuit, err)
	}
	if n := e.count(t, "SELECT count(*) FROM organizations WHERE id = $1 AND legal_name = 'Constructora Sur SA' AND doc_type = 'cuit'", p.OrgID); n != 1 {
		t.Fatal("verified identity not applied")
	}
	_, err = e.kyc.Decide(ctx, c.ID, kyc.Decision{Approved: false})
	wantCode(t, err, apperr.CodeKYCInvalidState)
	_, err = e.kyc.Decide(ctx, ids.New(), kyc.Decision{Approved: true})
	wantCode(t, err, apperr.CodeKYCCaseNotFound)
	_, err = e.kyc.Update(ctx, p, kyc.UpdateInput{Company: company("20111222")})
	wantCode(t, err, apperr.CodeKYCInvalidState)
	_, err = e.kyc.UploadDocument(ctx, p, kyc.DocumentUpload{Kind: kyc.DocOther, Content: strings.NewReader("%PDF-1")})
	wantCode(t, err, apperr.CodeKYCInvalidState)
}

func TestCompanyRejectedByOps(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 0)
	p := e.org(t, orgs.KindCompany)
	e.completeCompany(t, p)
	c, _ := e.kyc.Submit(t.Context(), p)
	reason := "bylaws_unreadable"
	c, err := e.kyc.Decide(t.Context(), c.ID, kyc.Decision{Approved: false, Reason: &reason})
	if err != nil || c.Status != kyc.StatusRejected || *c.DecisionReason != reason || e.orgStatus(t, p.OrgID) != "rejected" {
		t.Fatalf("reject %+v %v", c, err)
	}
	if len(e.hook.calls) != 0 {
		t.Fatal("rejection ran approval hooks")
	}
}

// DEV_KYC_AUTO_APPROVE_SECONDS: the submit enqueues a review job scheduled after the delay;
// the job approves the company.
func TestCompanyDevAutoApprove(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 8*time.Second)
	p := e.org(t, orgs.KindCompany)
	e.completeCompany(t, p)
	c, err := e.kyc.Submit(t.Context(), p)
	if err != nil || c.Status != kyc.StatusSubmitted {
		t.Fatalf("submit %+v %v", c, err)
	}
	var scheduled time.Time
	if err := e.pool.QueryRow(t.Context(), "SELECT scheduled_at FROM river_job WHERE kind = 'kyc_review' AND args->>'case_id' = $1",
		c.ID.String()).Scan(&scheduled); err != nil {
		t.Fatal(err)
	}
	if want := c.SubmittedAt.Add(8 * time.Second); !scheduled.Equal(want) {
		t.Fatalf("scheduled %v, want %v", scheduled, want)
	}
	if err := (&kyc.ReviewWorker{Service: e.kyc}).Work(t.Context(), &river.Job[kyc.ReviewArgs]{Args: kyc.ReviewArgs{CaseID: c.ID}}); err != nil {
		t.Fatal(err)
	}
	c, _ = e.kyc.Get(t.Context(), p)
	if c.Status != kyc.StatusApproved || *c.DecisionReason != "dev_auto_approve" || len(e.hook.calls) != 1 {
		t.Fatalf("auto-approved %+v hooks %v", c, e.hook.calls)
	}
}

// The hooks run INSIDE the approval transaction: if one fails, nothing is approved.
func TestApprovalHookFailureRollsBack(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 0)
	p := e.org(t, orgs.KindCompany)
	e.completeCompany(t, p)
	c, _ := e.kyc.Submit(t.Context(), p)
	e.hook.fail = errors.New("ledger down")
	if _, err := e.kyc.Decide(t.Context(), c.ID, kyc.Decision{Approved: true}); err == nil {
		t.Fatal("decide must fail with its hook")
	}
	c, _ = e.kyc.Get(t.Context(), p)
	if c.Status != kyc.StatusSubmitted || e.orgStatus(t, p.OrgID) != "submitted" {
		t.Fatalf("status %s / org %s after a failed hook", c.Status, e.orgStatus(t, p.OrgID))
	}
}

// A13: files are capped (10 MB passes, one byte more fails), type-sniffed and stored under the
// tenant's key.
func TestUploadRules(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 0)
	ctx := t.Context()
	p := e.org(t, orgs.KindPersonal)

	full := append([]byte("%PDF-1.7 "), bytes.Repeat([]byte{'a'}, kyc.MaxDocumentBytes-9)...)
	d, err := e.kyc.UploadDocument(ctx, p, kyc.DocumentUpload{Kind: kyc.DocOther, FileName: "../x.pdf", Content: bytes.NewReader(full)})
	if err != nil || d.SizeBytes != kyc.MaxDocumentBytes || d.ContentType != upload.TypePDF || d.FileName != "x.pdf" {
		t.Fatalf("10 MB upload %+v %v", d, err)
	}
	if b, ok := e.docs.Object("org/" + p.OrgID.String() + "/kyc/" + d.ID.String()); !ok || len(b) != kyc.MaxDocumentBytes {
		t.Fatal("object not stored under the tenant key")
	}
	_, err = e.kyc.UploadDocument(ctx, p, kyc.DocumentUpload{Kind: kyc.DocOther, Content: bytes.NewReader(append(full, 'a'))})
	wantCode(t, err, apperr.CodeBodyTooLarge)
	_, err = e.kyc.UploadDocument(ctx, p, kyc.DocumentUpload{
		Kind: kyc.DocSelfie, FileName: "selfie.png", ContentType: "image/png",
		Content: strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg"><script/></svg>`),
	})
	wantCode(t, err, apperr.CodeKYCDocumentInvalid)
	_, err = e.kyc.UploadDocument(ctx, p, kyc.DocumentUpload{Kind: "passport", Content: strings.NewReader("%PDF-1")})
	wantCode(t, err, apperr.CodeValidation)
	rep := "20111222"
	_, err = e.kyc.UploadDocument(ctx, p, kyc.DocumentUpload{Kind: kyc.DocSelfie, RepresentativeDNI: &rep, Content: strings.NewReader("%PDF-1")})
	wantCode(t, err, apperr.CodeValidation)
	_, err = e.kyc.Update(ctx, p, kyc.UpdateInput{Company: company("20111222")})
	wantCode(t, err, apperr.CodeValidation)
}

// T2: the case is found by the org of the principal; an org without a case is case_not_found.
func TestGetByOrg(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 0)
	a := e.org(t, orgs.KindCompany)
	b := e.org(t, orgs.KindPersonal)
	ca, _ := e.kyc.Get(t.Context(), a)
	cb, _ := e.kyc.Get(t.Context(), b)
	if ca.OrgID != a.OrgID || cb.OrgID != b.OrgID || ca.ID == cb.ID || ca.Kind != kyc.KindCompany || cb.Kind != kyc.KindPerson {
		t.Fatalf("cases %+v %+v", ca, cb)
	}
	_, err := e.kyc.Get(t.Context(), auth.Principal{OrgID: ids.New()})
	wantCode(t, err, apperr.CodeKYCCaseNotFound)
	if err := e.kyc.Review(t.Context(), ids.New()); err != nil {
		t.Fatalf("review of a missing case is a no-op: %v", err)
	}
}
