//go:build integration

package payouts_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"lenapay/apps/api/internal/accounts"
	"lenapay/apps/api/internal/fees"
	"lenapay/apps/api/internal/ledger"
	"lenapay/apps/api/internal/orgs"
	"lenapay/apps/api/internal/payouts"
	payoutfake "lenapay/apps/api/internal/payouts/provider/fake"
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
)

var pg *dbtest.Postgres

func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }

// Default ARS plan (fees.DefaultRates): payout = 150.00 + 0.20 %. On 10 000.00 the fee is
// 150.00 + 20.00 = 170.00, so 10 170.00 is held.
const (
	amountMinor = 1_000_000
	feeMinor    = 17_000
	totalMinor  = amountMinor + feeMinor
)

type env struct {
	pool      *pgxpool.Pool
	clock     *clock.Fake
	ledger    ledger.Service
	svc       payouts.Service
	psp       *fakePSP
	notifier  *recordingNotifier
	policies  *fakePolicies
	docs      *payoutfake.DiskDocumentStore
	org       uuid.UUID
	otherOrg  uuid.UUID
	creator   uuid.UUID
	approver  uuid.UUID
	account   uuid.UUID
	personal  uuid.UUID // personal org's account
	personOrg uuid.UUID
	master    uuid.UUID
	revenue   uuid.UUID
}

func setup(t *testing.T) *env {
	t.Helper()
	ctx := t.Context()
	pool := pg.NewDatabase(t)
	clk := clock.NewFake(time.Date(2026, 11, 7, 15, 0, 0, 0, time.UTC))
	led := ledger.NewService(ledger.Deps{Pool: pool, Clock: clk})
	if err := led.EnsureSystemAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	feeSvc := fees.NewService(fees.Deps{Pool: pool})
	if err := feeSvc.EnsureDefaultPlan(ctx); err != nil {
		t.Fatal(err)
	}
	e := &env{
		pool: pool, clock: clk, ledger: led, psp: newFakePSP(), notifier: &recordingNotifier{}, policies: &fakePolicies{},
		docs: payoutfake.NewDiskDocumentStore(t.TempDir()),
	}
	e.creator, e.approver = e.user(t), e.user(t)
	e.org, e.otherOrg, e.personOrg = e.newOrg(t, "company"), e.newOrg(t, "company"), e.newOrg(t, "personal")
	e.account = e.newAccount(t, e.org, "project")
	e.personal = e.newAccount(t, e.personOrg, "personal")
	var err error
	if e.master, err = led.SystemAccountID(ctx, ledger.SystemPSPMaster); err != nil {
		t.Fatal(err)
	}
	if e.revenue, err = led.SystemAccountID(ctx, ledger.SystemFeesRevenue); err != nil {
		t.Fatal(err)
	}
	inserter, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		t.Fatal(err)
	}
	e.svc = payouts.NewService(payouts.Deps{
		Pool: pool, Clock: clk, Audit: audit.NewLogger(clk), Idempotency: idempotency.NewStore(clk, idempotency.DefaultTTL),
		Jobs: inserter, PSP: e.psp, Ledger: led, Fees: feeSvc, Notifications: e.notifier, Documents: e.docs,
		Accounts: fakeAccounts{byID: map[uuid.UUID]accounts.Account{
			e.account:  {ID: e.account, OrgID: e.org, Kind: accounts.KindProject},
			e.personal: {ID: e.personal, OrgID: e.personOrg, Kind: accounts.KindPersonal},
		}},
		Orgs: fakeMembers{members: map[uuid.UUID][]orgs.Member{e.org: {
			{UserID: e.creator, Role: auth.RoleOperator}, {UserID: e.approver, Role: auth.RoleApprover},
		}}},
		Policies: e.policies,
	})
	return e
}

func (e *env) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := e.pool.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func (e *env) user(t *testing.T) uuid.UUID {
	id := ids.New()
	e.exec(t, `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, 'x')`, id, id.String()+"@test.local")
	return id
}

func (e *env) newOrg(t *testing.T, kind string) uuid.UUID {
	id := ids.New()
	e.exec(t, `INSERT INTO organizations (id, kind, legal_name, kyc_status, created_by) VALUES ($1, $2, 'Payouts Test SA', 'approved', $3)`,
		id, kind, e.creator)
	return id
}

func (e *env) newAccount(t *testing.T, orgID uuid.UUID, kind string) uuid.UUID {
	id := ids.New()
	e.exec(t, `INSERT INTO accounts (id, org_id, kind, name) VALUES ($1, $2, $3, 'Obra')`, id, orgID, kind)
	if err := db.WithTx(t.Context(), e.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return e.ledger.OpenAccount(t.Context(), tx, id)
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

// fund credits account like a payin would (psp_master → account).
func (e *env) fund(t *testing.T, account uuid.UUID, minor int64) {
	t.Helper()
	err := db.WithTx(t.Context(), e.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err := e.ledger.Post(t.Context(), tx, ledger.Entry{
			Kind: ledger.KindPayin, RefType: "payin", RefID: ids.New(),
			Legs: []ledger.Leg{{AccountID: e.master, Amount: money.New(-minor, money.ARS)}, {AccountID: account, Amount: money.New(minor, money.ARS)}},
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (e *env) balance(t *testing.T, account uuid.UUID) ledger.Balance {
	t.Helper()
	all, err := e.ledger.Balances(t.Context(), []uuid.UUID{account})
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range all {
		if b.Posted.Currency == money.ARS {
			return b
		}
	}
	t.Fatalf("no ARS balance for %s", account)
	return ledger.Balance{}
}

func assertBalance(t *testing.T, b ledger.Balance, posted, held int64) {
	t.Helper()
	if b.Posted.Minor != posted || b.Held.Minor != held {
		t.Fatalf("balance = posted %d held %d, want posted %d held %d", b.Posted.Minor, b.Held.Minor, posted, held)
	}
}

func (e *env) operator(org uuid.UUID) auth.Principal {
	return auth.Principal{UserID: e.creator, OrgID: org, Role: auth.RoleOperator, KYCStatus: auth.KYCApproved, StepUp: true}
}

func (e *env) approverP() auth.Principal {
	return auth.Principal{UserID: e.approver, OrgID: e.org, Role: auth.RoleApprover, KYCStatus: auth.KYCApproved, StepUp: true}
}

func createInput(account uuid.UUID, minor int64, key string) payouts.CreateInput {
	return payouts.CreateInput{
		AccountID: account, Amount: money.New(minor, money.ARS), To: "0170099220000067797370",
		Concept: "Pago certificado de obra 12", IdempotencyKey: key,
	}
}

func key() string { return "key-" + ids.New().String() }

func (e *env) jobs(t *testing.T, kind string) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(t.Context(), `SELECT count(*) FROM river_job WHERE kind = $1`, kind).Scan(&n); err != nil {
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

// R-OUT-2/R-OUT-4: a company payout holds amount + fee and waits for an approver.
func TestCreateHoldsAmountPlusFeeAndWaitsForApproval(t *testing.T) {
	e := setup(t)
	e.fund(t, e.account, 2_000_000)

	po, err := e.svc.Create(t.Context(), e.operator(e.org), createInput(e.account, amountMinor, key()))
	if err != nil {
		t.Fatal(err)
	}
	if po.Status != payouts.StatusPendingApproval || po.Fee.Minor != feeMinor || po.Destination.HolderCUIT != "30711122233" {
		t.Fatalf("payout = %+v", po)
	}
	if len(po.Events) != 1 || po.Events[0].ToStatus != payouts.StatusPendingApproval || po.Events[0].FromStatus != nil {
		t.Fatalf("events = %+v", po.Events)
	}
	assertBalance(t, e.balance(t, e.account), 2_000_000, totalMinor)
	if e.jobs(t, "dispatch_payout") != 0 {
		t.Fatal("a company payout must not be dispatched before approval")
	}
	if !slices.Contains(e.notifier.kinds(), "approval.requested") {
		t.Fatalf("notifications = %v, want approval.requested", e.notifier.kinds())
	}
	var audited int
	if err := e.pool.QueryRow(t.Context(), `SELECT count(*) FROM audit_log WHERE action = 'payout.create' AND target_id = $1`, po.ID).Scan(&audited); err != nil || audited != 1 {
		t.Fatalf("audit rows = %d (%v), want 1", audited, err)
	}
}

// Boundary pair (MO6): the last amount whose total fits is accepted, one cent more is not.
func TestCreateBoundaryAvailableBalance(t *testing.T) {
	e := setup(t)
	e.fund(t, e.account, totalMinor)
	_, err := e.svc.Create(t.Context(), e.operator(e.org), createInput(e.account, amountMinor+1, key()))
	wantCode(t, err, apperr.CodeInsufficientFunds)
	if _, err := e.svc.Create(t.Context(), e.operator(e.org), createInput(e.account, amountMinor, key())); err != nil {
		t.Fatalf("exact available balance must be accepted: %v", err)
	}
	assertBalance(t, e.balance(t, e.account), totalMinor, totalMinor)
}

// R-OUT-3 / MO2: same key + same body replays; same key + other body is 409.
func TestCreateIdempotencyKey(t *testing.T) {
	e := setup(t)
	e.fund(t, e.account, 5_000_000)
	k := key()
	first, err := e.svc.Create(t.Context(), e.operator(e.org), createInput(e.account, amountMinor, k))
	if err != nil {
		t.Fatal(err)
	}
	again, err := e.svc.Create(t.Context(), e.operator(e.org), createInput(e.account, amountMinor, k))
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != first.ID {
		t.Fatalf("replay created payout %s, want %s", again.ID, first.ID)
	}
	_, err = e.svc.Create(t.Context(), e.operator(e.org), createInput(e.account, amountMinor+100, k))
	wantCode(t, err, apperr.CodeIdempotencyReused)
	assertBalance(t, e.balance(t, e.account), 5_000_000, totalMinor) // one hold, not two
}

// MO3 / AG11: N parallel payouts against one balance → exactly the affordable ones, no overdraft.
func TestCreateConcurrentNeverOverdraws(t *testing.T) {
	e := setup(t)
	e.fund(t, e.account, 3*totalMinor+500)
	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := range n {
		wg.Go(func() {
			_, errs[i] = e.svc.Create(t.Context(), e.operator(e.org), createInput(e.account, amountMinor, key()))
		})
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case apperr.IsCode(err, apperr.CodeInsufficientFunds), apperr.IsCode(err, apperr.CodeOperationInProgress):
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if ok != 3 {
		t.Fatalf("%d payouts created, want exactly 3", ok)
	}
	assertBalance(t, e.balance(t, e.account), 3*totalMinor+500, 3*totalMinor)
}

// R-OUT-4: the creator cannot approve; an approver can, once, and that queues the dispatch.
func TestApproveIsFourEyes(t *testing.T) {
	e := setup(t)
	e.fund(t, e.account, 2_000_000)
	po, err := e.svc.Create(t.Context(), e.operator(e.org), createInput(e.account, amountMinor, key()))
	if err != nil {
		t.Fatal(err)
	}
	self := e.operator(e.org)
	self.Role = auth.RoleAdmin
	_, err = e.svc.Approve(t.Context(), self, po.ID, key())
	wantCode(t, err, apperr.CodePayoutSelfApproval)

	k := key()
	approved, err := e.svc.Approve(t.Context(), e.approverP(), po.ID, k)
	if err != nil {
		t.Fatal(err)
	}
	if approved.Status != payouts.StatusApproved || approved.ApprovedBy == nil || *approved.ApprovedBy != e.approver {
		t.Fatalf("approved = %+v", approved)
	}
	replay, err := e.svc.Approve(t.Context(), e.approverP(), po.ID, k)
	if err != nil || replay.Status != payouts.StatusApproved {
		t.Fatalf("replay = %+v, %v", replay, err)
	}
	_, err = e.svc.Approve(t.Context(), e.approverP(), po.ID, key())
	wantCode(t, err, apperr.CodePayoutInvalidState)
	if n := e.jobs(t, "dispatch_payout"); n != 1 {
		t.Fatalf("dispatch jobs = %d, want 1", n)
	}
}

// T2: another org's payout is not found, on read and on write.
func TestForeignPayoutIsNotFound(t *testing.T) {
	e := setup(t)
	e.fund(t, e.account, 2_000_000)
	po, err := e.svc.Create(t.Context(), e.operator(e.org), createInput(e.account, amountMinor, key()))
	if err != nil {
		t.Fatal(err)
	}
	foreign := e.approverP()
	foreign.OrgID = e.otherOrg
	_, err = e.svc.Get(t.Context(), foreign, po.ID)
	wantCode(t, err, apperr.CodePayoutNotFound)
	_, err = e.svc.Approve(t.Context(), foreign, po.ID, key())
	wantCode(t, err, apperr.CodePayoutNotFound)
	_, err = e.svc.Reject(t.Context(), foreign, po.ID, "no")
	wantCode(t, err, apperr.CodePayoutNotFound)
}

// MO5: reject and cancel release the hold (the money never left).
func TestRejectAndCancelReleaseTheHold(t *testing.T) {
	e := setup(t)
	e.fund(t, e.account, 3_000_000)
	a, err := e.svc.Create(t.Context(), e.operator(e.org), createInput(e.account, amountMinor, key()))
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.svc.Create(t.Context(), e.operator(e.org), createInput(e.account, amountMinor, key()))
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := e.svc.Reject(t.Context(), e.approverP(), a.ID, "Proveedor equivocado")
	if err != nil || rejected.Status != payouts.StatusRejected || *rejected.RejectReason != "Proveedor equivocado" {
		t.Fatalf("rejected = %+v, %v", rejected, err)
	}
	_, err = e.svc.Cancel(t.Context(), e.approverP(), b.ID) // not the creator
	wantCode(t, err, apperr.CodeForbidden)
	cancelled, err := e.svc.Cancel(t.Context(), e.operator(e.org), b.ID)
	if err != nil || cancelled.Status != payouts.StatusCancelled {
		t.Fatalf("cancelled = %+v, %v", cancelled, err)
	}
	_, err = e.svc.Cancel(t.Context(), e.operator(e.org), b.ID)
	wantCode(t, err, apperr.CodePayoutInvalidState)
	assertBalance(t, e.balance(t, e.account), 3_000_000, 0)
}

func (e *env) approved(t *testing.T) payouts.Payout {
	t.Helper()
	e.fund(t, e.account, 2_000_000)
	po, err := e.svc.Create(t.Context(), e.operator(e.org), createInput(e.account, amountMinor, key()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Approve(t.Context(), e.approverP(), po.ID, key()); err != nil {
		t.Fatal(err)
	}
	return po
}

func (e *env) get(t *testing.T, id uuid.UUID) payouts.Payout {
	t.Helper()
	po, err := e.svc.Get(t.Context(), e.operator(e.org), id)
	if err != nil {
		t.Fatal(err)
	}
	return po
}

// R-OUT-5 / R-FEE-1: settled captures the hold into ONE entry: account −total, master +amount,
// fees_revenue +fee; and the proof exists only then.
func TestDispatchSettledCapturesTheHold(t *testing.T) {
	e := setup(t)
	po := e.approved(t)
	_, err := e.svc.Proof(t.Context(), e.operator(e.org), po.ID)
	wantCode(t, err, apperr.CodePayoutProofUnavailable)

	if err := e.svc.Dispatch(t.Context(), po.ID); err != nil {
		t.Fatal(err)
	}
	got := e.get(t, po.ID)
	if got.Status != payouts.StatusSettled || got.SettledAt == nil || got.ProviderTransferID == nil || *got.ProviderTransferID != "bind-tr-1" {
		t.Fatalf("payout = %+v", got)
	}
	sent := e.psp.sent[0]
	if sent.Reference != fmt.Sprintf("po-%s-1", po.ID) || sent.Amount != money.New(amountMinor, money.ARS) || sent.AccountNumber != "0170099220000067797370" {
		t.Fatalf("sent = %+v", sent)
	}
	assertBalance(t, e.balance(t, e.account), 2_000_000-totalMinor, 0)
	assertBalance(t, e.balance(t, e.revenue), feeMinor, 0)
	if b := e.balance(t, e.master); b.Posted.Minor != -2_000_000+amountMinor {
		t.Fatalf("master posted = %d", b.Posted.Minor)
	}
	statuses := []payouts.Status{}
	for _, ev := range got.Events {
		statuses = append(statuses, ev.ToStatus)
	}
	if !slices.Equal(statuses, []payouts.Status{payouts.StatusPendingApproval, payouts.StatusApproved, payouts.StatusSending, payouts.StatusSettled}) {
		t.Fatalf("history = %v", statuses)
	}
	pdf, err := e.svc.Proof(t.Context(), e.operator(e.org), po.ID)
	if err != nil || !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) {
		t.Fatalf("proof: %v", err)
	}
	// A redelivered job finds the payout settled: no second transfer, no second posting.
	if err := e.svc.Dispatch(t.Context(), po.ID); err != nil || e.psp.sentCount() != 1 {
		t.Fatalf("redelivery: err %v, sent %d", err, e.psp.sentCount())
	}
	assertBalance(t, e.balance(t, e.account), 2_000_000-totalMinor, 0)
}

// R-OUT-5: a rejection releases the hold and fails the payout.
func TestDispatchRejectedReleasesTheHold(t *testing.T) {
	e := setup(t)
	po := e.approved(t)
	e.psp.script(psp.TransferResult{ProviderID: "bind-tr-9", Status: psp.TransferRejected, Reason: "closed account"}, nil, psp.TransferResult{}, nil)
	if err := e.svc.Dispatch(t.Context(), po.ID); err != nil {
		t.Fatal(err)
	}
	got := e.get(t, po.ID)
	if got.Status != payouts.StatusFailed || got.FailureCode == nil || *got.FailureCode != "psp.rejected" {
		t.Fatalf("payout = %+v", got)
	}
	assertBalance(t, e.balance(t, e.account), 2_000_000, 0)
}

// MO4 provider-down: an uncertain answer keeps the hold and the `sending` state; the sweep
// re-reads by OUR reference and settles through the same path, never sending twice.
func TestUncertainStaysSendingAndTheSweepSettles(t *testing.T) {
	e := setup(t)
	po := e.approved(t)
	e.psp.script(psp.TransferResult{}, psp.ErrUncertain, psp.TransferResult{}, psp.ErrUncertain)
	if err := e.svc.Dispatch(t.Context(), po.ID); err != nil {
		t.Fatal(err)
	}
	if got := e.get(t, po.ID); got.Status != payouts.StatusSending {
		t.Fatalf("status = %s, want sending", got.Status)
	}
	assertBalance(t, e.balance(t, e.account), 2_000_000, totalMinor)

	if err := e.svc.Sweep(t.Context()); err != nil { // too fresh: left to its own job
		t.Fatal(err)
	}
	if len(e.psp.lookups) != 0 {
		t.Fatalf("sweep re-read a fresh attempt: %v", e.psp.lookups)
	}
	e.clock.Advance(2 * time.Minute)
	if err := e.svc.Sweep(t.Context()); err != nil { // still uncertain: nothing changes
		t.Fatal(err)
	}
	assertBalance(t, e.balance(t, e.account), 2_000_000, totalMinor)

	e.psp.script(psp.TransferResult{}, nil, psp.TransferResult{ProviderID: "bind-tr-late", Status: psp.TransferSettled}, nil)
	if err := e.svc.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	got := e.get(t, po.ID)
	if got.Status != payouts.StatusSettled || *got.ProviderTransferID != "bind-tr-late" {
		t.Fatalf("payout = %+v", got)
	}
	if e.psp.sentCount() != 1 || e.psp.lookups[0] != fmt.Sprintf("po-%s-1", po.ID) {
		t.Fatalf("sent %d, lookups %v", e.psp.sentCount(), e.psp.lookups)
	}
	assertBalance(t, e.balance(t, e.account), 2_000_000-totalMinor, 0)
}

// MO4: "not found by our reference" releases the hold only after the grace period.
func TestSweepNotFoundFailsOnlyAfterGrace(t *testing.T) {
	e := setup(t)
	po := e.approved(t)
	e.psp.script(psp.TransferResult{}, psp.ErrUncertain, psp.TransferResult{}, psp.ErrNotFound)
	if err := e.svc.Dispatch(t.Context(), po.ID); err != nil {
		t.Fatal(err)
	}
	e.clock.Advance(14*time.Minute + 59*time.Second) // last instant still inside the grace
	if err := e.svc.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := e.get(t, po.ID); got.Status != payouts.StatusSending {
		t.Fatalf("status = %s, want sending inside the grace", got.Status)
	}
	e.clock.Advance(time.Second)
	if err := e.svc.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	got := e.get(t, po.ID)
	if got.Status != payouts.StatusFailed || *got.FailureCode != "psp.not_received" {
		t.Fatalf("payout = %+v", got)
	}
	assertBalance(t, e.balance(t, e.account), 2_000_000, 0)
}

// An unknown error is not "no" (A9): the payout stays sending and the job retries.
func TestUnknownProviderErrorKeepsSending(t *testing.T) {
	e := setup(t)
	po := e.approved(t)
	e.psp.script(psp.TransferResult{}, errors.New("socket closed"), psp.TransferResult{}, nil)
	if err := e.svc.Dispatch(t.Context(), po.ID); err == nil {
		t.Fatal("want the error returned so River retries the re-read")
	}
	if got := e.get(t, po.ID); got.Status != payouts.StatusSending {
		t.Fatalf("status = %s", got.Status)
	}
	assertBalance(t, e.balance(t, e.account), 2_000_000, totalMinor)
}

// R-OUT-4: personal payouts skip approval and are queued at once.
func TestPersonalPayoutIsSentDirectly(t *testing.T) {
	e := setup(t)
	e.fund(t, e.personal, 2_000_000)
	po, err := e.svc.Create(t.Context(), e.operator(e.personOrg), createInput(e.personal, amountMinor, key()))
	if err != nil {
		t.Fatal(err)
	}
	if po.Status != payouts.StatusApproved || e.jobs(t, "dispatch_payout") != 1 {
		t.Fatalf("status %s, jobs %d", po.Status, e.jobs(t, "dispatch_payout"))
	}
}

func TestListApprovalsAndSettledBetween(t *testing.T) {
	e := setup(t)
	e.fund(t, e.account, 5_000_000)
	for range 3 {
		if _, err := e.svc.Create(t.Context(), e.operator(e.org), createInput(e.account, amountMinor, key())); err != nil {
			t.Fatal(err)
		}
		e.clock.Advance(time.Second)
	}
	first, err := e.svc.ListApprovals(t.Context(), e.approverP(), page.Request{Limit: 2})
	if err != nil || len(first.Items) != 2 || first.NextCursor == nil {
		t.Fatalf("page 1 = %d items, cursor %v, %v", len(first.Items), first.NextCursor, err)
	}
	pos, err := page.Decode(*first.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	second, err := e.svc.ListApprovals(t.Context(), e.approverP(), page.Request{Limit: 2, After: &pos})
	if err != nil || len(second.Items) != 1 || second.NextCursor != nil {
		t.Fatalf("page 2 = %d items, %v", len(second.Items), err)
	}
	foreign := e.approverP()
	foreign.OrgID = e.otherOrg
	if other, err := e.svc.ListApprovals(t.Context(), foreign, page.Request{Limit: 10}); err != nil || len(other.Items) != 0 {
		t.Fatalf("foreign org sees %d approvals (%v)", len(other.Items), err)
	}

	if _, err := e.svc.Approve(t.Context(), e.approverP(), first.Items[0].ID, key()); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Dispatch(t.Context(), first.Items[0].ID); err != nil {
		t.Fatal(err)
	}
	now := e.clock.Now()
	settled, err := e.svc.SettledBetween(t.Context(), money.ARS, now.Add(-time.Minute), now.Add(time.Minute))
	if err != nil || len(settled) != 1 || settled[0].ID != first.Items[0].ID {
		t.Fatalf("settled = %+v, %v", settled, err)
	}
	if usd, err := e.svc.SettledBetween(t.Context(), money.USD, now.Add(-time.Minute), now.Add(time.Minute)); err != nil || len(usd) != 0 {
		t.Fatalf("USD settled = %d, %v", len(usd), err)
	}
}

// R-OUT-1: an unknown destination or a PSP outage stops the payout before any hold.
func TestDestinationErrorsHoldNothing(t *testing.T) {
	e := setup(t)
	e.fund(t, e.account, 2_000_000)
	e.psp.resolve = psp.ErrNotFound
	_, err := e.svc.Create(t.Context(), e.operator(e.org), createInput(e.account, amountMinor, key()))
	wantCode(t, err, apperr.CodeDestinationNotFound)
	e.psp.resolve = psp.ErrUncertain
	_, err = e.svc.Create(t.Context(), e.operator(e.org), createInput(e.account, amountMinor, key()))
	wantCode(t, err, apperr.CodeProviderUnavailable)
	assertBalance(t, e.balance(t, e.account), 2_000_000, 0)
}

func TestQuoteShowsFeeTotalAndAvailableAfter(t *testing.T) {
	e := setup(t)
	e.fund(t, e.account, 2_000_000)
	q, err := e.svc.Quote(t.Context(), e.operator(e.org), payouts.QuoteInput{AccountID: e.account, Amount: money.New(amountMinor, money.ARS), To: "proveedor.andes"})
	if err != nil {
		t.Fatal(err)
	}
	if q.Fee.Minor != feeMinor || q.Total.Minor != totalMinor || q.AvailableAfter.Minor != 2_000_000-totalMinor ||
		q.Destination.Type != psp.DestinationCBU || q.Destination.Value != "proveedor.andes" {
		t.Fatalf("quote = %+v", q)
	}
	assertBalance(t, e.balance(t, e.account), 2_000_000, 0) // a quote holds nothing
}
