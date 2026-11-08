//go:build integration

package payerview_test

// On-us payouts (P5) and the personal-account requirements, through the whole app: a payout
// to a Lenapay CVU or alias settles on the internal rail as ONE ledger entry (never through
// the PSP) and the receiver gets a credited payin of the same rail. With BINDMOCK_URL set
// these run against apps/bind-mock too.

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/accounts"
	"github.com/erfajc97/lenapay/apps/api/internal/orgs"
	"github.com/erfajc97/lenapay/apps/api/internal/payers"
	"github.com/erfajc97/lenapay/apps/api/internal/payouts"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
	"github.com/erfajc97/lenapay/apps/api/internal/psp/provider/bind"
)

// pspCalls counts the fake PSP's calls (0 in bind-mock mode: noBankTransfer checks there).
func (e *env) pspCalls() int {
	if e.fakePSP == nil {
		return 0
	}
	return e.fakePSP.Calls()
}

// noBankTransfer proves a payout never reached the bank: no attempt was recorded and, against
// the bind-mock, the bank does not know its first attempt reference.
func (e *env) noBankTransfer(t *testing.T, payoutID uuid.UUID) {
	t.Helper()
	if n := e.count(t, `SELECT count(*) FROM payouts WHERE id = $1 AND attempt_count = 0 AND provider_attempt_id IS NULL
		AND provider_transfer_id IS NULL`, payoutID); n != 1 {
		t.Fatalf("payout %s has a PSP attempt", payoutID)
	}
	if e.mockURL == "" {
		return
	}
	c := bind.New(bind.Options{BaseURL: e.mockURL, Timeout: 10 * time.Second})
	if _, err := c.TransferByReference(t.Context(), fmt.Sprintf("po-%s-1", payoutID)); !errors.Is(err, psp.ErrNotFound) {
		t.Fatalf("the bank knows on-us payout %s: %v", payoutID, err)
	}
}

// approver adds a second member who can approve the company's payouts (4-eyes).
func (e *env) approver(t *testing.T, c company, email string) auth.Principal {
	t.Helper()
	u := e.user(t, email)
	if _, err := e.svc.Orgs.AddMember(t.Context(), c.admin, orgs.MemberInput{Email: email, Role: auth.RoleApprover}); err != nil {
		t.Fatal(err)
	}
	return auth.Principal{UserID: u.id, SessionID: u.session, OrgID: c.admin.OrgID, Role: auth.RoleApprover, KYCStatus: auth.KYCApproved, StepUp: true}
}

// available is the account's available balance in ARS.
func (e *env) available(t *testing.T, orgID, accountID uuid.UUID) int64 {
	t.Helper()
	a, err := e.svc.Accounts.Get(t.Context(), orgID, accountID)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range a.Balances {
		if b.Available.Currency == money.ARS {
			return b.Available.Minor
		}
	}
	return 0
}

// personal is the approved person's principal and personal account.
func (e *env) personal(t *testing.T, u user, orgID uuid.UUID) (auth.Principal, uuid.UUID) {
	t.Helper()
	p := auth.Principal{UserID: u.id, SessionID: u.session, OrgID: orgID, Role: auth.RoleAdmin, KYCStatus: auth.KYCApproved, StepUp: true}
	list, err := e.svc.Accounts.List(t.Context(), p, false)
	if err != nil || len(list) != 1 || list[0].Kind != accounts.KindPersonal {
		t.Fatalf("personal accounts %v %v", list, err)
	}
	return p, list[0].ID
}

func payout(account uuid.UUID, minor int64, to string) payouts.CreateInput {
	return payouts.CreateInput{
		AccountID: account, Amount: money.New(minor, money.ARS), To: to, Concept: "Pago on-us",
		IdempotencyKey: "onus-" + ids.New().String(),
	}
}

// internalPayins counts the credited internal payins an org received for a payout.
func (e *env) internalPayins(t *testing.T, orgID, payoutID uuid.UUID, minor int64) int {
	t.Helper()
	return e.count(t, `SELECT count(*) FROM payins WHERE org_id = $1 AND payout_id = $2 AND rail = 'internal'
		AND status = 'credited' AND gross_minor = $3 AND net_minor = $3 AND fee_minor = 0`, orgID, payoutID, minor)
}

// Company → company: the payer's 4-eyes still applies (pending, nothing moves, no PSP call),
// then the approval settles instantly; the receiving company sees a payin in its movements.
func TestCompanyPaysAnotherCompanyOnUs(t *testing.T) {
	e := setup(t)
	a := e.company(t, "a@onus.test", "Alfa Obras SA", "30711111119")
	b := e.company(t, "b@onus.test", "Beta Hormigones SA", "30722222229")
	_, aCVU := e.payer(t, a, payers.DocDNI, "20333444")
	e.incoming(t, aCVU, 1_000_000_00, "Comprador Alfa")
	_, bCVU := e.payer(t, b, payers.DocDNI, "20555666")
	approver := e.approver(t, a, "approver@alfa.test")
	calls, before := e.pspCalls(), e.available(t, a.admin.OrgID, a.project)

	po, err := e.svc.Payouts.Create(t.Context(), a.admin, payout(a.project, 250_000_00, bCVU))
	if err != nil || po.Status != payouts.StatusPendingApproval || po.Rail != payouts.RailInternal || !po.Fee.IsZero() ||
		po.Destination.HolderName != "Beta Hormigones SA" || po.Destination.HolderCUIT != "30722222229" {
		t.Fatalf("create = %+v, %v", po, err)
	}
	if got := e.available(t, a.admin.OrgID, a.project); got != before-250_000_00 {
		t.Fatalf("held: available %d, want %d", got, before-250_000_00)
	}
	if n := e.internalPayins(t, b.admin.OrgID, po.ID, 250_000_00); n != 0 {
		t.Fatal("credited before the approval")
	}
	if _, err := e.svc.Payouts.Approve(t.Context(), a.admin, po.ID, "approve-"+ids.New().String()); !apperr.IsCode(err, apperr.CodePayoutSelfApproval) {
		t.Fatalf("self approval = %v", err)
	}
	approved, err := e.svc.Payouts.Approve(t.Context(), approver, po.ID, "approve-"+ids.New().String())
	if err != nil || approved.Status != payouts.StatusSettled || approved.SettledAt == nil {
		t.Fatalf("approve = %+v, %v", approved, err)
	}
	if e.pspCalls() != calls {
		t.Fatal("company → company on-us called the PSP")
	}
	e.noBankTransfer(t, po.ID)
	if n := e.internalPayins(t, b.admin.OrgID, po.ID, 250_000_00); n != 1 {
		t.Fatalf("receiving payins = %d", n)
	}
	if got := e.available(t, a.admin.OrgID, a.project); got != before-250_000_00 {
		t.Fatalf("payer available %d after settlement", got)
	}
	_, body := e.get(t, b.owner.token, fmt.Sprintf("/orgs/%s/movements?type=payin", b.admin.OrgID))
	found := false
	for _, m := range items(t, body) {
		if m["rail"] == "internal" && m["amount"].(map[string]any)["minor"] == float64(250_000_00) {
			found = m["counterparty"].(map[string]any)["name"] == "Alfa Obras SA"
		}
	}
	if !found {
		t.Fatalf("receiver movements %v", body)
	}
	_, body = e.get(t, a.owner.token, fmt.Sprintf("/orgs/%s/payouts/%s", a.admin.OrgID, po.ID))
	if body["rail"] != "internal" || body["status"] != "settled" {
		t.Fatalf("payout over HTTP = %v", body)
	}
}

// Company → person: paid to the person's personal alias; the person is credited, and a
// destination disabled before the approval fails the payout and releases the hold.
func TestCompanyPaysAPersonOnUs(t *testing.T) {
	e := setup(t)
	c := e.company(t, "c@onus.test", "Gamma Desarrollos SA", "30733333339")
	_, cCVU := e.payer(t, c, payers.DocDNI, "20777888")
	e.incoming(t, cCVU, 1_000_000_00, "Comprador Gamma")
	approver := e.approver(t, c, "approver@gamma.test")
	u, orgID := e.person(t, "sofia@persona.test", "31222333")
	pp, personalAccount := e.personal(t, u, orgID)
	alias := e.personalCVU(t, orgID).Alias
	if alias == nil {
		t.Fatal("personal CVU without alias")
	}
	calls := e.pspCalls()

	po, err := e.svc.Payouts.Create(t.Context(), c.admin, payout(c.project, 40_000_00, *alias))
	if err != nil || po.Rail != payouts.RailInternal || po.Destination.HolderCUIT != "31222333" {
		t.Fatalf("create = %+v, %v", po, err)
	}
	if _, err := e.svc.Payouts.Approve(t.Context(), approver, po.ID, "approve-"+ids.New().String()); err != nil {
		t.Fatal(err)
	}
	if n := e.internalPayins(t, orgID, po.ID, 40_000_00); n != 1 {
		t.Fatalf("person payins = %d", n)
	}
	if got := e.available(t, orgID, personalAccount); got != 40_000_00 {
		t.Fatalf("person available %d", got)
	}
	if e.pspCalls() != calls {
		t.Fatal("company → person on-us called the PSP")
	}

	// The destination stops receiving between creation and approval: failed, hold released.
	before := e.available(t, c.admin.OrgID, c.project)
	second, err := e.svc.Payouts.Create(t.Context(), c.admin, payout(c.project, 10_000_00, *alias))
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Payers.DisableCVU(t.Context(), pp, e.personalCVU(t, orgID).ID); err != nil {
		t.Fatal(err)
	}
	failed, err := e.svc.Payouts.Approve(t.Context(), approver, second.ID, "approve-"+ids.New().String())
	if err != nil || failed.Status != payouts.StatusFailed || failed.FailureCode == nil || *failed.FailureCode != "destination_unavailable" {
		t.Fatalf("approve after disable = %+v, %v", failed, err)
	}
	if got := e.available(t, c.admin.OrgID, c.project); got != before {
		t.Fatalf("hold not released: %d, want %d", got, before)
	}
}

// Concurrency and idempotency: N payouts at once against a balance that covers exactly 3 →
// exactly 3 settle and the rest are insufficient_funds; one key sent twice at once is one
// payout and one receiving payin. The PSP is never called.
func TestOnUsPayoutsUnderConcurrency(t *testing.T) {
	e := setup(t)
	c := e.company(t, "d@onus.test", "Delta Viviendas SA", "30744444449")
	_, cCVU := e.payer(t, c, payers.DocDNI, "20999000")
	u, orgID := e.person(t, "martin@persona.test", "32444555")
	pp, account := e.personal(t, u, orgID)
	e.incoming(t, e.personalCVU(t, orgID).Number, 1_000_000_00, "Martin")
	amount := e.available(t, orgID, account) / 3
	calls := e.pspCalls()

	const n = 8
	var wg sync.WaitGroup
	results := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, results[i] = e.svc.Payouts.Create(t.Context(), pp, payout(account, amount, cCVU))
		}()
	}
	wg.Wait()
	ok, short := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			ok++
		case apperr.IsCode(err, apperr.CodeInsufficientFunds):
			short++
		default:
			t.Fatalf("unexpected error %v", err)
		}
	}
	if ok != 3 || short != n-3 {
		t.Fatalf("settled %d, insufficient %d", ok, short)
	}
	if got := e.count(t, `SELECT count(*) FROM payins WHERE org_id = $1 AND rail = 'internal'`, c.admin.OrgID); got != 3 {
		t.Fatalf("receiving payins = %d", got)
	}
	if got := e.available(t, orgID, account); got < 0 || got >= amount {
		t.Fatalf("personal available %d after 3 × %d", got, amount)
	}

	// The same key twice at once: one payout, one payin.
	if e.pspCalls() != calls {
		t.Fatal("on-us payouts called the PSP")
	}
	e.incoming(t, e.personalCVU(t, orgID).Number, 100_000_00, "Martin")
	calls = e.pspCalls()
	in := payout(account, 1_000_00, cCVU)
	ids2 := make([]uuid.UUID, 2)
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			po, err := e.svc.Payouts.Create(t.Context(), pp, in)
			if err != nil {
				t.Errorf("replay %d: %v", i, err)
				return
			}
			ids2[i] = po.ID
		}()
	}
	wg.Wait()
	if ids2[0] != ids2[1] || e.internalPayins(t, c.admin.OrgID, ids2[0], 1_000_00) != 1 {
		t.Fatalf("same key: payouts %v", ids2)
	}
	if e.pspCalls() != calls {
		t.Fatal("on-us payouts called the PSP")
	}
}

// Rules of the internal rail: same account and currency mismatch are refused before any hold;
// reconciliation never sees internal items (they never touch the master account).
func TestOnUsRulesAndReconciliation(t *testing.T) {
	e := setup(t)
	c := e.company(t, "e@onus.test", "Epsilon Obras SA", "30755555559")
	_, cCVU := e.payer(t, c, payers.DocDNI, "21000111")
	u, orgID := e.person(t, "ana@persona.test", "33555666")
	pp, account := e.personal(t, u, orgID)
	own := e.personalCVU(t, orgID)
	e.incoming(t, own.Number, 500_000_00, "Ana")

	if _, err := e.svc.Payouts.Create(t.Context(), pp, payout(account, 1_000_00, own.Number)); !apperr.IsCode(err, apperr.CodeDestinationInvalid) {
		t.Fatalf("to own CVU = %v", err)
	}
	usd := payout(account, 1_000_00, cCVU)
	usd.Amount = money.New(1_000_00, money.USD)
	if _, err := e.svc.Payouts.Create(t.Context(), pp, usd); !apperr.IsCode(err, apperr.CodeCurrencyMismatch) {
		t.Fatalf("USD to an ARS CVU = %v", err)
	}
	q, err := e.svc.Payouts.Quote(t.Context(), pp, payouts.QuoteInput{AccountID: account, Amount: money.New(1_000_00, money.ARS), To: cCVU})
	if err != nil || q.Rail != payouts.RailInternal || !q.Fee.IsZero() {
		t.Fatalf("quote = %+v, %v", q, err)
	}
	po, err := e.svc.Payouts.Create(t.Context(), pp, payout(account, 7_000_00, cCVU))
	if err != nil || po.Status != payouts.StatusSettled {
		t.Fatalf("create = %+v, %v", po, err)
	}
	from, to := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	settled, err := e.svc.Payouts.SettledBetween(t.Context(), money.ARS, from, to)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range settled {
		if s.ID == po.ID {
			t.Fatal("reconciliation lists an internal payout")
		}
	}
	credited, err := e.svc.Payins.CreditedBetween(t.Context(), money.ARS, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(credited) != 1 { // only Ana's funding payin through the bank
		t.Fatalf("reconciliation payins = %d", len(credited))
	}
}

// Signup gives every user a personal org (KYC draft) with its personal account and no CVUs;
// the person KYC approval queues the CVU job, and her accounts then list her ARS and USD CVUs,
// whose alias she changes; a payer's CVU or another org's CVU is not hers (404).
func TestPersonalAccountAtSignupAndItsCVUs(t *testing.T) {
	e := setup(t)
	u := e.user(t, "nueva@persona.test")
	o := e.personalOrg(t, u.id)
	if o.KYCStatus != auth.KYCDraft || o.LegalName != "nueva@persona.test" {
		t.Fatalf("personal org = %+v", o)
	}
	status, body := e.get(t, u.token, "/orgs/"+o.ID.String()+"/accounts")
	list := items(t, body)
	if status != http.StatusOK || len(list) != 1 || list[0]["kind"] != "personal" || len(list[0]["cvus"].([]any)) != 0 {
		t.Fatalf("draft accounts = %d %v", status, body)
	}
	if _, err := e.svc.Orgs.Create(t.Context(), auth.Principal{UserID: u.id}, orgs.CreateInput{Kind: orgs.KindPersonal, LegalName: "Otra"}); !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("second personal org = %v", err)
	}

	p, orgID := e.person(t, "lucia@persona.test", dniLucia)
	if n := e.count(t, `SELECT count(*) FROM river_job WHERE kind = 'payers.create_personal_cvus' AND args->>'org_id' = $1`, orgID.String()); n != 1 {
		t.Fatalf("personal CVU jobs = %d", n)
	}
	path := "/orgs/" + orgID.String() + "/accounts"
	_, body = e.get(t, p.token, path)
	acc := items(t, body)[0]
	cvus := acc["cvus"].([]any)
	if len(cvus) != 2 {
		t.Fatalf("cvus = %v", cvus)
	}
	first := cvus[0].(map[string]any)
	keys(t, "account cvu", first, "id", "currency", "cvu", "alias", "status", "share")
	patch := fmt.Sprintf("%s/%s/cvus/%s", path, acc["id"], first["id"])
	headers := map[string]string{}
	status, body = e.do(t, call{method: http.MethodPatch, path: patch, token: p.token, body: map[string]any{"alias": "lucia.pagos"}, headers: headers})
	if status != http.StatusOK || body["alias"] != "lucia.pagos" || body["id"] != first["id"] {
		t.Fatalf("patch alias = %d %v", status, body)
	}

	c := e.company(t, "f@onus.test", "Zeta Obras SA", "30766666669")
	payerID, _ := e.payer(t, c, payers.DocDNI, "21222333")
	py, err := e.svc.Payers.Get(t.Context(), c.admin, payerID)
	if err != nil {
		t.Fatal(err)
	}
	payerCVU := fmt.Sprintf("/orgs/%s/accounts/%s/cvus/%s", c.admin.OrgID, c.project, py.CVUs[0].ID)
	status, body = e.do(t, call{method: http.MethodPatch, path: payerCVU, token: c.owner.token, body: map[string]any{"alias": "zeta.payer1"}})
	wantError(t, status, body, http.StatusNotFound, string(apperr.CodeCVUNotFound))
	foreign := fmt.Sprintf("%s/%s/cvus/%s", path, acc["id"], py.CVUs[0].ID)
	status, body = e.do(t, call{method: http.MethodPatch, path: foreign, token: p.token, body: map[string]any{"alias": "zeta.payer2"}})
	wantError(t, status, body, http.StatusNotFound, string(apperr.CodeCVUNotFound))
	status, body = e.do(t, call{method: http.MethodPatch, path: patch, token: c.owner.token, body: map[string]any{"alias": "zeta.payer3"}})
	if status != http.StatusNotFound && status != http.StatusForbidden {
		t.Fatalf("another org's user = %d %v", status, body)
	}
}
