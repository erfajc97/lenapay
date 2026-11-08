//go:build integration

package payers_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"lenapay/apps/api/internal/accounts"
	"lenapay/apps/api/internal/ledger"
	"lenapay/apps/api/internal/orgs"
	"lenapay/apps/api/internal/payers"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/audit"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/clock"
	"lenapay/apps/api/internal/platform/db"
	"lenapay/apps/api/internal/platform/db/dbtest"
	"lenapay/apps/api/internal/platform/idempotency"
	"lenapay/apps/api/internal/platform/ids"
	"lenapay/apps/api/internal/platform/money"
	"lenapay/apps/api/internal/platform/page"
	"lenapay/apps/api/internal/psp"
	pspfake "lenapay/apps/api/internal/psp/provider/fake"
)

var (
	pg      *dbtest.Postgres
	cuitSeq atomic.Int64
)

func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }

// recorder is the jobs.Enqueuer of the tests: it records what a transaction queued.
type recorder struct {
	mu   sync.Mutex
	jobs []river.JobArgs
}

func (r *recorder) InsertTx(_ context.Context, _ pgx.Tx, args river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs = append(r.jobs, args)
	return &rivertype.JobInsertResult{}, nil
}

// orgDirectory answers orgs.Get / ListMembers from the rows the test inserted.
type orgDirectory struct{ pool *pgxpool.Pool }

func (d orgDirectory) Get(ctx context.Context, p auth.Principal) (orgs.Organization, error) {
	var o orgs.Organization
	err := d.pool.QueryRow(ctx, `SELECT o.id, o.legal_name, o.cuit FROM organizations o JOIN memberships m ON m.org_id = o.id
		WHERE o.id = $1 AND m.user_id = $2`, p.OrgID, p.UserID).Scan(&o.ID, &o.LegalName, &o.CUIT)
	if errors.Is(err, pgx.ErrNoRows) {
		return o, apperr.New(apperr.CodeOrgNotFound, nil)
	}
	return o, err
}

func (d orgDirectory) ListMembers(ctx context.Context, p auth.Principal) ([]orgs.Member, error) {
	rows, err := d.pool.Query(ctx, `SELECT user_id FROM memberships WHERE org_id = $1 ORDER BY created_at`, p.OrgID)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	out := make([]orgs.Member, 0, len(ids))
	for _, id := range ids {
		out = append(out, orgs.Member{UserID: id})
	}
	return out, err
}

type env struct {
	pool     *pgxpool.Pool
	svc      payers.Service
	accounts accounts.Service
	psp      *pspfake.Provider
	jobs     *recorder
	p, other auth.Principal
	account  uuid.UUID // project account of p's org
}

func setup(t *testing.T) env {
	t.Helper()
	pool := pg.NewDatabase(t)
	clk := clock.NewFake(time.Date(2026, 11, 7, 15, 0, 0, 0, time.UTC))
	led := ledger.NewService(ledger.Deps{Pool: pool, Clock: clk})
	if err := led.EnsureSystemAccounts(t.Context()); err != nil {
		t.Fatal(err)
	}
	idem := idempotency.NewStore(clk, idempotency.DefaultTTL)
	acc := accounts.NewService(accounts.Deps{Pool: pool, Clock: clk, Audit: audit.NewLogger(clk), Idempotency: idem, Ledger: led})
	e := env{pool: pool, accounts: acc, psp: pspfake.New(), jobs: &recorder{}}
	e.svc = payers.NewService(payers.Deps{
		Pool: pool, Clock: clk, Audit: audit.NewLogger(clk), Idempotency: idem, Jobs: e.jobs, PSP: e.psp,
		Accounts: acc, Orgs: orgDirectory{pool: pool}, ShareBaseURL: "https://lenapay.test/pagar/",
	})
	e.p, e.other = e.newOrg(t, "company"), e.newOrg(t, "company")
	a, err := acc.Create(t.Context(), e.p, accounts.CreateInput{Name: "Torre Norte"})
	if err != nil {
		t.Fatal(err)
	}
	e.account = a.ID
	return e
}

func (e env) newOrg(t *testing.T, kind string) auth.Principal {
	t.Helper()
	user, org := ids.New(), ids.New()
	e.exec(t, `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, 'x')`, user, user.String()+"@t.local")
	cuit := fmt.Sprintf("30%09d", cuitSeq.Add(1)) // one approved org per CUIT (DB unique)
	e.exec(t, `INSERT INTO organizations (id, kind, legal_name, cuit, kyc_status, created_by) VALUES ($1, $2, 'Constructora Sur SA', $3, 'approved', $4)`, org, kind, cuit, user)
	e.exec(t, `INSERT INTO memberships (org_id, user_id, role) VALUES ($1, $2, 'admin')`, org, user)
	return auth.Principal{UserID: user, SessionID: ids.New(), OrgID: org, Role: auth.RoleAdmin, KYCStatus: auth.KYCApproved, StepUp: true}
}

func (e env) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := e.pool.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func (e env) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(t.Context(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

func (e env) payer(t *testing.T, name string) payers.Payer {
	t.Helper()
	email := "  buyer@example.com "
	p, err := e.svc.Create(t.Context(), e.p, payers.CreateInput{AccountID: e.account, Name: "  " + name + " ", DocType: payers.DocDNI, DocNumber: "30111222", Email: &email})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPayerCRUDAndTenancy(t *testing.T) {
	t.Parallel()
	e := setup(t)
	ctx := t.Context()
	created := e.payer(t, "Juan Pérez")
	if created.Name != "Juan Pérez" || *created.Email != "buyer@example.com" || created.AccountID != e.account || len(created.CVUs) != 0 {
		t.Fatalf("created %+v", created)
	}
	if e.count(t, `SELECT count(*) FROM audit_log WHERE action = 'payer.create' AND target_id = $1`, created.ID) != 1 {
		t.Fatal("create not audited")
	}
	name := "Juan P. Pérez"
	updated, err := e.svc.Update(ctx, e.p, created.ID, payers.UpdateInput{Name: &name})
	if err != nil || updated.Name != name || *updated.Email != "buyer@example.com" { // absent email keeps the stored one (A4)
		t.Fatalf("updated %+v %v", updated, err)
	}
	short := " J "
	if _, err := e.svc.Update(ctx, e.p, created.ID, payers.UpdateInput{Name: &short}); !apperr.IsCode(err, apperr.CodeValidation) {
		t.Fatalf("name shorter than 2 after trim: %v", err)
	}
	twoChars := "Jo"
	if _, err := e.svc.Update(ctx, e.p, created.ID, payers.UpdateInput{Name: &twoChars}); err != nil {
		t.Fatalf("2 characters is the minimum: %v", err)
	}
	// "email": null clears it; a new email sets it again.
	cleared, err := e.svc.Update(ctx, e.p, created.ID, payers.UpdateInput{ClearEmail: true})
	if err != nil || cleared.Email != nil || cleared.Name != twoChars {
		t.Fatalf("clear email %+v %v", cleared, err)
	}
	other := "otro@example.com"
	if again, err := e.svc.Update(ctx, e.p, created.ID, payers.UpdateInput{Email: &other}); err != nil || again.Email == nil || *again.Email != other {
		t.Fatalf("set email %+v %v", again, err)
	}

	// T2: another org cannot read, change or issue CVUs for this payer; it answers like a missing one.
	if _, err := e.svc.Get(ctx, e.other, created.ID); !apperr.IsCode(err, apperr.CodePayerNotFound) {
		t.Fatalf("foreign get: %v", err)
	}
	if _, err := e.svc.Update(ctx, e.other, created.ID, payers.UpdateInput{Name: &name}); !apperr.IsCode(err, apperr.CodePayerNotFound) {
		t.Fatalf("foreign update: %v", err)
	}
	if _, err := e.svc.CreateCVU(ctx, e.other, created.ID, payers.CreateCVUInput{Currency: money.ARS, IdempotencyKey: "foreign-key-0000000001"}); !apperr.IsCode(err, apperr.CodePayerNotFound) {
		t.Fatalf("foreign cvu: %v", err)
	}
	if list, _ := e.svc.List(ctx, e.other, payers.ListInput{Page: page.Request{Limit: 20}}); len(list.Items) != 0 {
		t.Fatal("foreign list leaks")
	}
	// T6: a payer cannot be created under another org's account.
	if _, err := e.svc.Create(ctx, e.other, payers.CreateInput{AccountID: e.account, Name: "X Y", DocType: payers.DocDNI, DocNumber: "30111222"}); !apperr.IsCode(err, apperr.CodeAccountNotFound) {
		t.Fatalf("foreign account: %v", err)
	}
}

func TestListSearchesAndPages(t *testing.T) {
	t.Parallel()
	e := setup(t)
	for _, n := range []string{"Ana Gómez", "Bruno Díaz", "Carla Gómez"} {
		e.payer(t, n)
	}
	q := "gómez"
	first, err := e.svc.List(t.Context(), e.p, payers.ListInput{Query: &q, Page: page.Request{Limit: 1}})
	if err != nil || len(first.Items) != 1 || first.NextCursor == nil {
		t.Fatalf("first page %+v %v", first, err)
	}
	after, _ := page.Decode(*first.NextCursor)
	second, err := e.svc.List(t.Context(), e.p, payers.ListInput{Query: &q, Page: page.Request{Limit: 1, After: &after}})
	if err != nil || len(second.Items) != 1 || second.NextCursor != nil || second.Items[0].ID == first.Items[0].ID {
		t.Fatalf("second page %+v %v", second, err)
	}
	doc := "30111222"
	if all, _ := e.svc.List(t.Context(), e.p, payers.ListInput{Query: &doc, Page: page.Request{Limit: 20}}); len(all.Items) != 3 {
		t.Fatalf("search by document: %d", len(all.Items))
	}
}

// MO2: the same key and request replays the first CVU; another request with the key is 409;
// a second CVU in the currency is refused; the PSP is never asked for a key that cannot succeed.
func TestCreateCVUIsIdempotent(t *testing.T) {
	t.Parallel()
	e := setup(t)
	ctx := t.Context()
	payer := e.payer(t, "Juan Pérez")
	in := payers.CreateCVUInput{Currency: money.ARS, IdempotencyKey: "client-key-0000000001"}
	first, err := e.svc.CreateCVU(ctx, e.p, payer.ID, in)
	if err != nil || len(first.Number) != 22 || !first.Active || first.AccountID != e.account || *first.PayerID != payer.ID ||
		first.ShareLink != "https://lenapay.test/pagar/"+first.Number || first.QRPayload != first.ShareLink+"?currency=ARS" {
		t.Fatalf("first %+v %v", first, err)
	}
	again, err := e.svc.CreateCVU(ctx, e.p, payer.ID, in)
	if err != nil || again.ID != first.ID || len(e.psp.CVUs()) != 1 {
		t.Fatalf("replay %+v %v (psp cvus %d)", again, err, len(e.psp.CVUs()))
	}
	calls := e.psp.Calls()
	if _, err := e.svc.CreateCVU(ctx, e.p, payer.ID, payers.CreateCVUInput{Currency: money.USD, IdempotencyKey: in.IdempotencyKey}); !apperr.IsCode(err, apperr.CodeIdempotencyReused) || e.psp.Calls() != calls {
		t.Fatalf("key reused: %v (psp calls %d → %d)", err, calls, e.psp.Calls())
	}
	if _, err := e.svc.CreateCVU(ctx, e.p, payer.ID, payers.CreateCVUInput{Currency: money.ARS, IdempotencyKey: "client-key-0000000002"}); !apperr.IsCode(err, apperr.CodeCVUAlreadyExists) {
		t.Fatalf("second ARS cvu: %v", err)
	}
	if e.count(t, `SELECT count(*) FROM audit_log WHERE action = 'cvu.create' AND idempotency_key = $1`, in.IdempotencyKey) != 1 {
		t.Fatal("cvu creation audited once with its key")
	}
	got, err := e.svc.Get(ctx, e.p, payer.ID)
	if err != nil || len(got.CVUs) != 1 || got.CVUs[0].ID != first.ID {
		t.Fatalf("payer cvus %+v %v", got.CVUs, err)
	}
	// Concurrent first requests with one key: one CVU, the same answer for both.
	key := "client-key-0000000003"
	var wg sync.WaitGroup
	results := make([]payers.CVU, 4)
	errs := make([]error, 4)
	for i := range results {
		wg.Go(func() {
			results[i], errs[i] = e.svc.CreateCVU(ctx, e.p, payer.ID, payers.CreateCVUInput{Currency: money.USD, IdempotencyKey: key})
		})
	}
	wg.Wait()
	for i := range results {
		if errs[i] != nil || results[i].ID != results[0].ID {
			t.Fatalf("concurrent %d: %+v %v", i, results[i], errs[i])
		}
	}
	if e.count(t, `SELECT count(*) FROM cvus WHERE payer_id = $1 AND currency = 'USD'`, payer.ID) != 1 {
		t.Fatal("two USD CVUs")
	}
}

// MO4: an uncertain PSP answer is a 503 that changes nothing; the retry with the same key
// sends the same reference, so the PSP can return the CVU it may have created.
func TestCreateCVUWhenThePSPIsDown(t *testing.T) {
	t.Parallel()
	e := setup(t)
	payer := e.payer(t, "Juan Pérez")
	in := payers.CreateCVUInput{Currency: money.ARS, IdempotencyKey: "client-key-0000000001"}
	e.psp.FailNext(psp.ErrUncertain)
	if _, err := e.svc.CreateCVU(t.Context(), e.p, payer.ID, in); !apperr.IsCode(err, apperr.CodeProviderUnavailable) {
		t.Fatalf("uncertain: %v", err)
	}
	if e.count(t, `SELECT count(*) FROM cvus`) != 0 || e.count(t, `SELECT count(*) FROM idempotency_keys`) != 0 {
		t.Fatal("an uncertain answer must leave no CVU and no claimed key")
	}
	cvu, err := e.svc.CreateCVU(t.Context(), e.p, payer.ID, in)
	if err != nil || !cvu.Active {
		t.Fatalf("retry: %+v %v", cvu, err)
	}
	alias := "taken.alias"
	e.psp.FailNext(psp.ErrAliasTaken)
	if _, err := e.svc.CreateCVU(t.Context(), e.p, payer.ID, payers.CreateCVUInput{Currency: money.USD, Alias: &alias, IdempotencyKey: "client-key-0000000002"}); !apperr.IsCode(err, apperr.CodeCVUAliasTaken) {
		t.Fatalf("alias taken at the PSP: %v", err)
	}
}

func TestAliasAndDisable(t *testing.T) {
	t.Parallel()
	e := setup(t)
	ctx := t.Context()
	payer := e.payer(t, "Juan Pérez")
	cvu, err := e.svc.CreateCVU(ctx, e.p, payer.ID, payers.CreateCVUInput{Currency: money.ARS, IdempotencyKey: "client-key-0000000001"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := e.svc.UpdateCVUAlias(ctx, e.p, cvu.ID, "torre.norte.1")
	if err != nil || *updated.Alias != "torre.norte.1" || e.psp.CVUs()[firstRef(e)].Alias != "torre.norte.1" {
		t.Fatalf("alias %+v %v", updated, err)
	}
	if _, err := e.svc.UpdateCVUAlias(ctx, e.p, cvu.ID, "no spaces"); !apperr.IsCode(err, apperr.CodeValidation) {
		t.Fatalf("bad alias: %v", err)
	}
	if _, err := e.svc.UpdateCVUAlias(ctx, e.other, cvu.ID, "foreign.alias"); !apperr.IsCode(err, apperr.CodeCVUNotFound) {
		t.Fatalf("foreign alias change: %v", err)
	}
	if err := e.svc.DisableCVU(ctx, e.other, cvu.ID); !apperr.IsCode(err, apperr.CodeCVUNotFound) {
		t.Fatalf("foreign disable: %v", err)
	}
	if err := e.svc.DisableCVU(ctx, e.p, cvu.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.DisableCVU(ctx, e.p, cvu.ID); err != nil {
		t.Fatalf("disabling twice is a no-op: %v", err)
	}
	if _, err := e.svc.UpdateCVUAlias(ctx, e.p, cvu.ID, "otro.alias"); !apperr.IsCode(err, apperr.CodeCVUDisabled) {
		t.Fatalf("alias of a disabled cvu: %v", err)
	}
	if e.count(t, `SELECT count(*) FROM audit_log WHERE action = 'cvu.disable'`) != 1 {
		t.Fatal("disable audited once")
	}
	// A disabled CVU frees the currency: the payer can get a new one.
	if _, err := e.svc.CreateCVU(ctx, e.p, payer.ID, payers.CreateCVUInput{Currency: money.ARS, IdempotencyKey: "client-key-0000000002"}); err != nil {
		t.Fatalf("new cvu after disable: %v", err)
	}
	if got, err := e.svc.CVUByNumber(ctx, cvu.Number); err != nil || got.ID != cvu.ID || got.Active {
		t.Fatalf("by number %+v %v", got, err)
	}
	if _, err := e.svc.CVUByNumber(ctx, "0000000000000000000000"); !apperr.IsCode(err, apperr.CodeCVUNotFound) {
		t.Fatalf("unknown number: %v", err)
	}
}

func firstRef(e env) string {
	for ref := range e.psp.CVUs() {
		return ref
	}
	return ""
}

func TestImportPayerIsIdempotent(t *testing.T) {
	t.Parallel()
	e := setup(t)
	in := payers.ImportInput{AccountID: e.account, Name: "Comprador ERP", DocType: payers.DocCUIT, DocNumber: "20123456786", ERPRef: "erp-client-42"}
	first, err := e.svc.ImportPayer(t.Context(), e.p.OrgID, in)
	if err != nil || first.ERPRef == nil || *first.ERPRef != "erp-client-42" {
		t.Fatalf("import %+v %v", first, err)
	}
	again, err := e.svc.ImportPayer(t.Context(), e.p.OrgID, in)
	if err != nil || again.ID != first.ID || e.count(t, `SELECT count(*) FROM payers`) != 1 {
		t.Fatalf("re-import %+v %v", again, err)
	}
	if _, err := e.svc.ImportPayer(t.Context(), e.other.OrgID, in); !apperr.IsCode(err, apperr.CodeAccountNotFound) {
		t.Fatalf("import under a foreign account: %v", err)
	}
}

// R-CVU-3: a personal org gets one CVU per currency after KYC approval, queued in the approval
// transaction and created by the job; running the job again creates nothing.
func TestPersonalCVUs(t *testing.T) {
	t.Parallel()
	e := setup(t)
	ctx := t.Context()
	person := e.newOrg(t, "personal")
	err := db.WithTx(ctx, e.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := e.accounts.OnKYCApproved(ctx, tx, person.OrgID, "personal"); err != nil {
			return err
		}
		if err := e.svc.OnKYCApproved(ctx, tx, person.OrgID, "personal"); err != nil {
			return err
		}
		return e.svc.OnKYCApproved(ctx, tx, e.p.OrgID, "company") // companies get no personal CVUs
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.jobs.jobs) != 1 || e.jobs.jobs[0].(payers.PersonalCVUsArgs).OrgID != person.OrgID {
		t.Fatalf("jobs %+v", e.jobs.jobs)
	}
	for range 2 {
		if err := e.svc.CreatePersonalCVUs(ctx, person.OrgID); err != nil {
			t.Fatal(err)
		}
	}
	if n := e.count(t, `SELECT count(*) FROM cvus WHERE org_id = $1 AND payer_id IS NULL AND status = 'active'`, person.OrgID); n != 2 {
		t.Fatalf("personal cvus = %d", n)
	}
	if len(e.psp.CVUs()) != 2 {
		t.Fatalf("psp cvus = %d", len(e.psp.CVUs()))
	}
}
