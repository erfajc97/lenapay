//go:build integration

package reconciliation_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/erfajc97/lenapay/apps/api/internal/payers"
	"github.com/erfajc97/lenapay/apps/api/internal/payins"
	"github.com/erfajc97/lenapay/apps/api/internal/payouts"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db/dbtest"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/page"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
	"github.com/erfajc97/lenapay/apps/api/internal/reconciliation"
)

var pg *dbtest.Postgres

func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }

// fakes: the PSP statement and transfers, credited payins, settled payouts and CVU owners.
type fakePSP struct {
	psp.Provider // only Statement and GetTransfer are called; anything else panics loudly
	lines        []psp.StatementLine
	transfers    map[string]psp.IncomingTransfer
	statementDay time.Time
}

func (f *fakePSP) Statement(_ context.Context, _ money.Currency, day time.Time) ([]psp.StatementLine, error) {
	f.statementDay = day
	return f.lines, nil
}

func (f *fakePSP) GetTransfer(_ context.Context, id string) (psp.IncomingTransfer, error) {
	tr, ok := f.transfers[id]
	if !ok {
		return psp.IncomingTransfer{}, psp.ErrNotFound
	}
	return tr, nil
}

type fakePayins struct{ items []payins.Payin }

func (f fakePayins) CreditedBetween(_ context.Context, c money.Currency, from, to time.Time) ([]payins.Payin, error) {
	var out []payins.Payin
	for _, p := range f.items {
		if p.Gross.Currency == c && p.CreditedAt != nil && !p.CreditedAt.Before(from) && p.CreditedAt.Before(to) {
			out = append(out, p)
		}
	}
	return out, nil
}

type fakePayouts struct{ items []payouts.Payout }

func (f fakePayouts) SettledBetween(_ context.Context, c money.Currency, from, to time.Time) ([]payouts.Payout, error) {
	var out []payouts.Payout
	for _, p := range f.items {
		if p.Amount.Currency == c && p.SettledAt != nil && !p.SettledAt.Before(from) && p.SettledAt.Before(to) {
			out = append(out, p)
		}
	}
	return out, nil
}

type fakeCVUs map[string]uuid.UUID

func (f fakeCVUs) CVUByNumber(_ context.Context, number string) (payers.CVU, error) {
	org, ok := f[number]
	if !ok {
		return payers.CVU{}, apperr.New(apperr.CodeCVUNotFound, nil)
	}
	return payers.CVU{OrgID: org, Number: number}, nil
}

type env struct {
	pool       *pgxpool.Pool
	svc        reconciliation.Service
	psp        *fakePSP
	orgA, orgB uuid.UUID
}

var art = time.FixedZone("ART", -3*3600)

func setup(t *testing.T) *env {
	t.Helper()
	pool := pg.NewDatabase(t)
	e := &env{pool: pool}
	user := ids.New()
	e.exec(t, `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, 'x')`, user, user.String()+"@test.local")
	e.orgA, e.orgB = ids.New(), ids.New()
	for _, org := range []uuid.UUID{e.orgA, e.orgB} {
		e.exec(t, `INSERT INTO organizations (id, kind, legal_name, kyc_status, created_by) VALUES ($1, 'company', 'Recon SA', 'approved', $2)`, org, user)
	}
	// Business day 2026-11-06 in ART is [06 03:00Z, 07 03:00Z).
	inDay := time.Date(2026, 11, 6, 22, 0, 0, 0, time.UTC)
	nextDay := time.Date(2026, 11, 7, 4, 0, 0, 0, time.UTC)
	e.psp = &fakePSP{
		lines: []psp.StatementLine{
			{TransferID: "in-a1", Direction: psp.DirectionIn, Amount: money.New(100_000, money.ARS)},
			{TransferID: "po-b-1", Direction: psp.DirectionOut, Amount: money.New(40_000, money.ARS)},
			{TransferID: "in-orphan", Direction: psp.DirectionIn, Amount: money.New(5_000, money.ARS)},
			{TransferID: "in-unknown", Direction: psp.DirectionIn, Amount: money.New(7_000, money.ARS)},
		},
		transfers: map[string]psp.IncomingTransfer{"in-orphan": {ID: "in-orphan", CVU: "0000003100099999999901"}},
	}
	attempt := "po-b-1"
	inside, outside := inDay, nextDay
	inserter, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		t.Fatal(err)
	}
	e.svc = reconciliation.NewService(reconciliation.Deps{
		Pool: pool, Clock: clock.NewFake(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)), Jobs: inserter, PSP: e.psp,
		Location: art, CVUs: fakeCVUs{"0000003100099999999901": e.orgA},
		Payins: fakePayins{items: []payins.Payin{
			{OrgID: e.orgA, ProviderTransferID: "in-a1", Gross: money.New(100_000, money.ARS), CreditedAt: &inside},
			{OrgID: e.orgB, ProviderTransferID: "in-b-late", Gross: money.New(1_000, money.ARS), CreditedAt: &outside},
			{OrgID: e.orgB, ProviderTransferID: "in-b-missing", Gross: money.New(2_500, money.ARS), CreditedAt: &inside},
		}},
		Payouts: fakePayouts{items: []payouts.Payout{
			{OrgID: e.orgB, Amount: money.New(40_000, money.ARS), ProviderAttemptID: &attempt, SettledAt: &inside},
		}},
	})
	return e
}

func (e *env) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := e.pool.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func (e *env) run(t *testing.T, org *uuid.UUID) reconciliation.Run {
	t.Helper()
	day := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	r, err := e.svc.RequestRun(t.Context(), reconciliation.RunInput{OrgID: org, Currency: money.ARS, Day: day})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != reconciliation.StatusQueued {
		t.Fatalf("new run status = %s", r.Status)
	}
	if err := e.svc.Reconcile(t.Context(), r.ID); err != nil {
		t.Fatal(err)
	}
	return e.list(t, e.orgOrNil(org), r.ID)
}

func (e *env) orgOrNil(org *uuid.UUID) uuid.UUID {
	if org == nil {
		return uuid.Nil
	}
	return *org
}

// list reads back the run (ops runs are not listed to orgs, so read them from the table).
func (e *env) list(t *testing.T, org uuid.UUID, id uuid.UUID) reconciliation.Run {
	t.Helper()
	if org == uuid.Nil {
		r := reconciliation.Run{ID: id}
		var psp, ledger, diff int64
		err := e.pool.QueryRow(t.Context(), `SELECT status, matched_count, missing_in_ledger_count, missing_at_psp_count,
			psp_total_minor, ledger_total_minor, difference_minor FROM reconciliation_runs WHERE id = $1`, id).
			Scan(&r.Status, &r.MatchedCount, &r.MissingInLedgerCount, &r.MissingAtPSPCount, &psp, &ledger, &diff)
		if err != nil {
			t.Fatal(err)
		}
		r.PSPTotal, r.LedgerTotal, r.Difference = money.New(psp, money.ARS), money.New(ledger, money.ARS), money.New(diff, money.ARS)
		return r
	}
	pg, err := e.svc.ListRuns(t.Context(), admin(org), nil, page.Request{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range pg.Items {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("run %s not listed for its org", id)
	return reconciliation.Run{}
}

func admin(org uuid.UUID) auth.Principal {
	return auth.Principal{UserID: ids.New(), OrgID: org, Role: auth.RoleAdmin, KYCStatus: auth.KYCApproved}
}

// R-REC-1, ops run: the whole master account of the business day.
func TestOpsRunCoversTheWholeMasterAccount(t *testing.T) {
	e := setup(t)
	r := e.run(t, nil)
	// matched in-a1 and po-b-1; missing_in_ledger in-orphan, in-unknown; missing_at_psp in-b-missing.
	if r.Status != reconciliation.StatusCompleted || r.MatchedCount != 2 || r.MissingInLedgerCount != 2 || r.MissingAtPSPCount != 1 {
		t.Fatalf("run = %+v", r)
	}
	// PSP: 100 000 − 40 000 + 5 000 + 7 000 = 72 000; ledger: 100 000 − 40 000 + 2 500 = 62 500.
	if r.PSPTotal.Minor != 72_000 || r.LedgerTotal.Minor != 62_500 || r.Difference.Minor != 9_500 {
		t.Fatalf("totals = %+v", r)
	}
	if !e.psp.statementDay.Equal(time.Date(2026, 11, 6, 0, 0, 0, 0, art)) {
		t.Fatalf("statement asked for %s", e.psp.statementDay)
	}
}

// Decision (lane D): an org run only sees that org's movements, including a PSP line we never
// credited when its receiving CVU is the org's.
func TestOrgRunSeesOnlyItsOwnMovements(t *testing.T) {
	e := setup(t)
	a := e.run(t, &e.orgA)
	if a.MatchedCount != 1 || a.MissingInLedgerCount != 1 || a.MissingAtPSPCount != 0 || a.PSPTotal.Minor != 105_000 || a.Difference.Minor != 5_000 {
		t.Fatalf("org A run = %+v", a)
	}
	b := e.run(t, &e.orgB)
	if b.MatchedCount != 1 || b.MissingInLedgerCount != 0 || b.MissingAtPSPCount != 1 || b.LedgerTotal.Minor != -37_500 {
		t.Fatalf("org B run = %+v", b)
	}
	var orgItems int
	if err := e.pool.QueryRow(t.Context(), `SELECT count(*) FROM reconciliation_items WHERE run_id = $1 AND org_id IS DISTINCT FROM $2`, a.ID, e.orgA).Scan(&orgItems); err != nil || orgItems != 0 {
		t.Fatalf("org A run stores %d foreign items (%v)", orgItems, err)
	}
	// T2: org A lists only its runs; ops runs are never listed.
	e.run(t, nil)
	pg, err := e.svc.ListRuns(t.Context(), admin(e.orgA), nil, page.Request{Limit: 10})
	if err != nil || len(pg.Items) != 1 || pg.Items[0].ID != a.ID {
		t.Fatalf("org A lists %+v (%v)", pg.Items, err)
	}
	usd := money.USD
	if pg, err := e.svc.ListRuns(t.Context(), admin(e.orgA), &usd, page.Request{Limit: 10}); err != nil || len(pg.Items) != 0 {
		t.Fatalf("USD filter lists %d runs (%v)", len(pg.Items), err)
	}
	viewer := admin(e.orgA)
	viewer.Role = auth.RoleApprover
	if _, err := e.svc.ListRuns(t.Context(), viewer, nil, page.Request{Limit: 10}); !apperr.IsCode(err, apperr.CodeForbidden) {
		t.Fatalf("non-admin err = %v", err)
	}
}

// A redelivered job finds the run completed and writes nothing twice.
func TestReconcileIsIdempotent(t *testing.T) {
	e := setup(t)
	r := e.run(t, nil)
	var before, after int
	_ = e.pool.QueryRow(t.Context(), `SELECT count(*) FROM reconciliation_items WHERE run_id = $1`, r.ID).Scan(&before)
	if err := e.svc.Reconcile(t.Context(), r.ID); err != nil {
		t.Fatal(err)
	}
	_ = e.pool.QueryRow(t.Context(), `SELECT count(*) FROM reconciliation_items WHERE run_id = $1`, r.ID).Scan(&after)
	if before != 5 || after != before {
		t.Fatalf("items before %d, after %d", before, after)
	}
}

// The daily job creates yesterday's ops run and one run per active org, once, however often it fires.
func TestScheduleDailyCreatesEachRunOnce(t *testing.T) {
	e := setup(t)
	for range 2 {
		if err := e.svc.ScheduleDaily(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	var ars, usd, jobs int
	_ = e.pool.QueryRow(t.Context(), `SELECT count(*) FROM reconciliation_runs WHERE currency = 'ARS' AND day = '2026-11-06' AND requested_by = 'schedule'`).Scan(&ars)
	_ = e.pool.QueryRow(t.Context(), `SELECT count(*) FROM reconciliation_runs WHERE currency = 'USD' AND requested_by = 'schedule'`).Scan(&usd)
	_ = e.pool.QueryRow(t.Context(), `SELECT count(*) FROM river_job WHERE kind = 'reconcile_master'`).Scan(&jobs)
	if ars != 3 || usd != 1 || jobs != 4 { // ARS: ops + orgs A and B; USD: ops only
		t.Fatalf("ARS runs %d, USD runs %d, jobs %d", ars, usd, jobs)
	}
}

func TestRequestRunRejectsFutureDays(t *testing.T) {
	e := setup(t)
	_, err := e.svc.RequestRun(t.Context(), reconciliation.RunInput{Currency: money.ARS, Day: time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)})
	if !apperr.IsCode(err, apperr.CodeValidation) {
		t.Fatalf("err = %v", err)
	}
	if _, err := e.svc.RequestRun(t.Context(), reconciliation.RunInput{Currency: money.ARS, Day: time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("today must be accepted: %v", err)
	}
}
