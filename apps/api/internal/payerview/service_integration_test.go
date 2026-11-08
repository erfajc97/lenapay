//go:build integration

package payerview_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/payers"
	"github.com/erfajc97/lenapay/apps/api/internal/payouts"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
)

const (
	dniLucia = "30123456"
	andina   = "Constructora Andina SRL"
	andinaCU = "30715532014" // erp-org-002 in the ERP fixtures
)

// The KYC approval itself links the matching payers (inside the approval transaction): the
// DNI as a DNI, as the middle digits of a CUIT and of a CUIL; a neighbour DNI never matches.
func TestKYCApprovalLinksPayersByDNIAndCUITMiddleDigits(t *testing.T) {
	e := setup(t)
	a := e.company(t, "a@empresa.test", "Alfa Desarrollos SA", "30700000011")
	b := e.company(t, "b@empresa.test", "Beta Obras SRL", "30700000022")
	e.payer(t, a, payers.DocDNI, dniLucia)
	e.payer(t, b, payers.DocCUIT, "27"+dniLucia+"8")
	e.payer(t, b, payers.DocCUIL, "27"+dniLucia+"9")
	e.payer(t, a, payers.DocDNI, "30123457")           // neighbour DNI
	e.payer(t, a, payers.DocCUIT, "20"+"30123457"+"1") // neighbour CUIT
	e.payer(t, a, payers.DocDNI, "130123456")          // 9 digits: not a DNI that pads to it

	_, orgID := e.person(t, "lucia@persona.test", dniLucia)

	rows, err := e.pool.Query(t.Context(), `SELECT matched_doc_type FROM payer_links WHERE personal_org_id = $1 ORDER BY matched_doc_type`, orgID)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		got = append(got, s)
	}
	rows.Close()
	if len(got) != 3 || got[0] != "cuil" || got[1] != "cuit" || got[2] != "dni" {
		t.Fatalf("links after approval = %v, want [cuil cuit dni]", got)
	}
	if n := e.count(t, `SELECT count(*) FROM audit_log WHERE org_id = $1 AND action = 'payer_link.create'`, orgID); n != 3 {
		t.Fatalf("audit entries = %d", n)
	}
}

// A 7-digit DNI matches a CUIT that carries it zero-padded, and a payer DNI with or without
// the leading zero.
func TestSevenDigitDNIMatchesPaddedDocuments(t *testing.T) {
	e := setup(t)
	a := e.company(t, "a@empresa.test", "Alfa Desarrollos SA", "30700000011")
	e.payer(t, a, payers.DocCUIT, "20081234565")
	e.payer(t, a, payers.DocDNI, "8123456")
	e.payer(t, a, payers.DocDNI, "08123456")
	u, _ := e.person(t, "abuela@persona.test", "8123456")
	status, body := e.get(t, u.token, "/me/payments/companies")
	if status != http.StatusOK || len(items(t, body)) != 3 {
		t.Fatalf("status %d body %v", status, body)
	}
}

// Payers created after the approval are linked on the next read (lazy matching).
func TestPayersCreatedLaterAreLinkedOnRead(t *testing.T) {
	e := setup(t)
	u, orgID := e.person(t, "lucia@persona.test", dniLucia)
	status, body := e.get(t, u.token, "/me/payments/companies")
	if status != http.StatusOK || len(items(t, body)) != 0 {
		t.Fatalf("before: %d %v", status, body)
	}
	a := e.company(t, "a@empresa.test", "Alfa Desarrollos SA", "30700000011")
	e.payer(t, a, payers.DocDNI, dniLucia)
	status, body = e.get(t, u.token, "/me/payments/companies")
	if status != http.StatusOK || len(items(t, body)) != 1 {
		t.Fatalf("after: %d %v", status, body)
	}
	e.get(t, u.token, "/me/payments/companies") // a second read links nothing twice
	if n := e.count(t, `SELECT count(*) FROM payer_links WHERE personal_org_id = $1`, orgID); n != 1 {
		t.Fatalf("links = %d", n)
	}
}

// Unverified data never matches: a KYC case in draft, a company's self-declared CUIT, or an
// approved personal org without a verified document (approved before the identity was copied).
func TestUnverifiedDataNeverMatches(t *testing.T) {
	e := setup(t)
	a := e.company(t, "a@empresa.test", "Alfa Desarrollos SA", "30700000011")
	e.payer(t, a, payers.DocDNI, dniLucia)

	draft, _ := e.personDraft(t, "draft@persona.test", dniLucia)
	status, body := e.get(t, draft.token, "/me/payments/companies")
	wantError(t, status, body, http.StatusForbidden, string(apperr.CodeKYCNotApproved))

	// The company owner's own org carries a CUIT around the DNI: a company is not a person.
	owner := e.company(t, "self@empresa.test", "Lucia Inversiones", "27"+dniLucia+"8")
	status, body = e.get(t, owner.owner.token, "/me/payments/companies")
	wantError(t, status, body, http.StatusForbidden, string(apperr.CodeKYCNotApproved))

	legacy, legacyOrg := e.person(t, "legacy@persona.test", "29999888")
	if _, err := e.pool.Exec(t.Context(), `UPDATE organizations SET doc_type = NULL, doc_number = NULL WHERE id = $1`, legacyOrg); err != nil {
		t.Fatal(err)
	}
	e.payer(t, a, payers.DocDNI, "29999888")
	status, body = e.get(t, legacy.token, "/me/payments/companies")
	wantError(t, status, body, http.StatusForbidden, string(apperr.CodeKYCNotApproved))

	if n := e.count(t, `SELECT count(*) FROM payer_links`); n != 0 {
		t.Fatalf("links from unverified data = %d", n)
	}
	if status, _ := e.get(t, "", "/me/payments/companies"); status != http.StatusUnauthorized {
		t.Fatalf("anonymous = %d", status)
	}
}

// The attacker: another verified person never sees, pages, hides or pays someone else's
// company; every door answers exactly like a missing link (404).
func TestAnotherPersonsCompanyIsNotFoundEverywhere(t *testing.T) {
	e := setup(t)
	a := e.company(t, "a@empresa.test", "Alfa Desarrollos SA", "30700000011")
	_, cvu := e.payer(t, a, payers.DocDNI, dniLucia)
	e.incoming(t, cvu, 50_000_00, "Lucia")
	victim, _ := e.person(t, "lucia@persona.test", dniLucia)
	attacker, attackerOrg := e.person(t, "mallory@persona.test", "31555666")
	e.incoming(t, e.personalCVU(t, attackerOrg), 1_000_000_00, "Mallory")

	_, body := e.get(t, victim.token, "/me/payments/companies")
	linkID := items(t, body)[0]["linkId"]

	status, body := e.get(t, attacker.token, "/me/payments/companies?includeHidden=true")
	if status != http.StatusOK || len(items(t, body)) != 0 {
		t.Fatalf("attacker list = %d %v", status, body)
	}
	payBody := map[string]any{"amount": map[string]any{"minor": 1000, "currency": "ARS"}}
	payHeaders := map[string]string{"Idempotency-Key": "attack-" + ids.New().String(), auth.HeaderStepUp: e.stepUp(t, attacker)}
	for name, c := range map[string]call{
		"payins":       {method: http.MethodGet, path: companiesPath(linkID, "/payins")},
		"installments": {method: http.MethodGet, path: companiesPath(linkID, "/installments")},
		"hide":         {method: http.MethodPatch, path: companiesPath(linkID, ""), body: map[string]any{"hidden": true}},
		"pay":          {method: http.MethodPost, path: companiesPath(linkID, "/pay"), body: payBody, headers: payHeaders},
	} {
		c.token = attacker.token
		status, body := e.do(t, c)
		if status != http.StatusNotFound || errCode(body) != string(apperr.CodePayerLinkNotFound) {
			t.Errorf("%s: %d %v", name, status, body)
		}
	}
	if n := e.count(t, `SELECT count(*) FROM payouts WHERE org_id = $1`, attackerOrg); n != 0 {
		t.Fatalf("attacker payouts = %d", n)
	}
	_, body = e.get(t, victim.token, "/me/payments/companies")
	if hidden := items(t, body)[0]["hidden"]; hidden != false {
		t.Fatalf("the victim's company was hidden by the attacker: %v", hidden)
	}
	if status, body := e.get(t, victim.token, companiesPath(uuid.New(), "/payins")); status != http.StatusNotFound || errCode(body) != string(apperr.CodePayerLinkNotFound) {
		t.Fatalf("unknown link = %d %v", status, body)
	}
}

// Every response is an allowlist: exact key sets, and nothing of the company beyond its name,
// the project and the person's own CVUs and payins.
func TestEndpointsAnswerExactKeySets(t *testing.T) {
	e := setup(t)
	a := e.company(t, "a@empresa.test", "Alfa Desarrollos SA", "30700000011")
	_, cvu := e.payer(t, a, payers.DocDNI, dniLucia)
	e.incoming(t, cvu, 1_500_000_00, "Lucia F")
	e.incoming(t, cvu, 250_000_50, "Lucia F")
	u, _ := e.person(t, "lucia@persona.test", dniLucia)

	status, body := e.get(t, u.token, "/me/payments/companies")
	if status != http.StatusOK {
		t.Fatalf("list = %d %v", status, body)
	}
	keys(t, "list", body, "items")
	c := items(t, body)[0]
	keys(t, "company", c, "linkId", "companyName", "projectName", "cvus", "totalPaid", "lastPaymentAt", "erpLinked", "hidden")
	if c["companyName"] != "Alfa Desarrollos SA" || c["projectName"] != "Torre Alfa" || c["erpLinked"] != false || c["lastPaymentAt"] == nil {
		t.Fatalf("company = %v", c)
	}
	cvus := c["cvus"].([]any)
	if len(cvus) != 1 {
		t.Fatalf("cvus = %v", cvus)
	}
	keys(t, "cvu", cvus[0], "currency", "cvu", "alias")
	if cvus[0].(map[string]any)["cvu"] != cvu {
		t.Fatalf("cvu = %v, want %s", cvus[0], cvu)
	}
	totals := c["totalPaid"].([]any)
	keys(t, "total", totals[0], "minor", "currency")
	if len(totals) != 1 || totals[0].(map[string]any)["minor"] != float64(1_750_000_50) {
		t.Fatalf("totals = %v", totals)
	}

	status, body = e.get(t, u.token, companiesPath(c["linkId"], "/payins?limit=1"))
	if status != http.StatusOK {
		t.Fatalf("payins = %d %v", status, body)
	}
	keys(t, "payins", body, "items", "nextCursor")
	first := items(t, body)[0]
	keys(t, "payin", first, "id", "cvu", "amount", "status", "createdAt", "creditedAt")
	if first["status"] != "credited" || first["amount"].(map[string]any)["minor"] != float64(250_000_50) || body["nextCursor"] == nil {
		t.Fatalf("newest payin = %v (cursor %v)", first, body["nextCursor"])
	}
	status, body = e.get(t, u.token, companiesPath(c["linkId"], "/payins?limit=1&cursor="+body["nextCursor"].(string)))
	if status != http.StatusOK || items(t, body)[0]["amount"].(map[string]any)["minor"] != float64(1_500_000_00) || body["nextCursor"] != nil {
		t.Fatalf("second page = %d %v", status, body)
	}

	status, body = e.get(t, u.token, companiesPath(c["linkId"], "/installments"))
	if status != http.StatusOK {
		t.Fatalf("installments = %d %v", status, body)
	}
	keys(t, "installments", body, "erpLinked", "items")
	if body["erpLinked"] != false || len(items(t, body)) != 0 {
		t.Fatalf("installments of a company without ERP = %v", body)
	}
}

// Hide and show: a hidden company leaves the list (a read never un-hides it) and comes back.
func TestHideAndShowACompany(t *testing.T) {
	e := setup(t)
	a := e.company(t, "a@empresa.test", "Alfa Desarrollos SA", "30700000011")
	e.payer(t, a, payers.DocDNI, dniLucia)
	u, orgID := e.person(t, "lucia@persona.test", dniLucia)
	_, body := e.get(t, u.token, "/me/payments/companies")
	linkID := items(t, body)[0]["linkId"]

	status, body := e.do(t, call{method: http.MethodPatch, path: companiesPath(linkID, ""), token: u.token, body: map[string]any{"hidden": true}})
	if status != http.StatusOK || body["hidden"] != true {
		t.Fatalf("hide = %d %v", status, body)
	}
	keys(t, "patched", body, "linkId", "companyName", "projectName", "cvus", "totalPaid", "lastPaymentAt", "erpLinked", "hidden")
	if _, body = e.get(t, u.token, "/me/payments/companies"); len(items(t, body)) != 0 {
		t.Fatalf("hidden still listed: %v", body)
	}
	if _, body = e.get(t, u.token, "/me/payments/companies?includeHidden=true"); len(items(t, body)) != 1 || items(t, body)[0]["hidden"] != true {
		t.Fatalf("includeHidden: %v", body)
	}
	status, body = e.do(t, call{method: http.MethodPatch, path: companiesPath(linkID, ""), token: u.token, body: map[string]any{"hidden": false}})
	if status != http.StatusOK || body["hidden"] != false {
		t.Fatalf("show = %d %v", status, body)
	}
	if n := e.count(t, `SELECT count(*) FROM audit_log WHERE org_id = $1 AND action IN ('payer_link.hidden', 'payer_link.active')`, orgID); n != 2 {
		t.Fatalf("audit = %d", n)
	}
	status, body = e.do(t, call{method: http.MethodPatch, path: companiesPath(linkID, ""), token: u.token, body: map[string]any{"hidden": true, "orgId": uuid.NewString()}})
	wantError(t, status, body, http.StatusUnprocessableEntity, string(apperr.CodeValidation))
}

// andinaWithLucia is the ERP-linked company whose ERP buyer carries Lucía's CUIT (fixture
// erp-org-002-buyer-005, 12 CAC installments), with an ARS CVU for her payer.
func andinaWithLucia(t *testing.T, e *env) (company, string) {
	t.Helper()
	c := e.company(t, "obras@andina.test", andina, andinaCU)
	e.linkERP(t, c, "obras@constructoraandina.com.ar")
	var payerID uuid.UUID
	if err := e.pool.QueryRow(t.Context(), `SELECT id FROM payers WHERE org_id = $1 AND erp_ref = 'erp-org-002-buyer-005'`, c.admin.OrgID).Scan(&payerID); err != nil {
		t.Fatalf("imported payer: %v", err)
	}
	cvu, err := e.svc.Payers.CreateCVU(t.Context(), c.admin, payerID, payers.CreateCVUInput{Currency: money.ARS, IdempotencyKey: "cvu-" + ids.New().String()})
	if err != nil {
		t.Fatal(err)
	}
	return c, cvu.Number
}

func TestInstallmentsOfAnERPLinkedCompany(t *testing.T) {
	e := setup(t)
	andinaWithLucia(t, e)
	u, _ := e.person(t, "lucia@persona.test", dniLucia)
	_, body := e.get(t, u.token, "/me/payments/companies")
	c := companyNamed(t, items(t, body), andina)
	if c["erpLinked"] != true || c["projectName"] != "Edificio Mendoza Centro" {
		t.Fatalf("company = %v", c)
	}
	status, body := e.get(t, u.token, companiesPath(c["linkId"], "/installments"))
	if status != http.StatusOK || body["erpLinked"] != true {
		t.Fatalf("installments = %d %v", status, body)
	}
	plan := items(t, body)
	if len(plan) != 12 {
		t.Fatalf("plan = %d items", len(plan))
	}
	keys(t, "installment", plan[0], "number", "dueDate", "amount", "status", "adjustmentIndex")
	first := plan[0]
	if first["number"] != float64(1) || first["dueDate"] != "2026-07-10" || first["status"] != "paid" || first["adjustmentIndex"] != "CAC" ||
		first["amount"].(map[string]any)["minor"] != float64(85_000_000) || first["amount"].(map[string]any)["currency"] != "ARS" {
		t.Fatalf("first installment = %v", first)
	}
	if plan[3]["status"] != "overdue" || plan[11]["status"] != "pending" {
		t.Fatalf("statuses = %v / %v", plan[3]["status"], plan[11]["status"])
	}
}

// The pay flow end to end: step-up and Idempotency-Key, the installment in the concept, a
// replay, the rule errors, then the dispatch settles at the PSP and the personal balance pays
// amount + fee. With BINDMOCK_URL set this runs against apps/bind-mock.
func TestPayACompanyEndToEnd(t *testing.T) {
	e := setup(t)
	_, companyCVU := andinaWithLucia(t, e)
	u, orgID := e.person(t, "lucia@persona.test", dniLucia)
	e.incoming(t, e.personalCVU(t, orgID), 2_000_000_00, "Lucia Fernandez")
	_, body := e.get(t, u.token, "/me/payments/companies")
	linkID := companyNamed(t, items(t, body), andina)["linkId"]
	path := companiesPath(linkID, "/pay")
	pay := map[string]any{"amount": map[string]any{"minor": 85_000_000, "currency": "ARS"}, "installmentNumber": 4}
	key := "pay-" + ids.New().String()

	status, body := e.do(t, call{method: http.MethodPost, path: path, token: u.token, body: pay, headers: map[string]string{"Idempotency-Key": key}})
	wantError(t, status, body, http.StatusForbidden, string(apperr.CodeStepUpRequired))

	headers := map[string]string{"Idempotency-Key": key, auth.HeaderStepUp: e.stepUp(t, u)}
	status, body = e.do(t, call{method: http.MethodPost, path: path, token: u.token, body: pay, headers: headers})
	if status != http.StatusCreated {
		t.Fatalf("pay = %d %v", status, body)
	}
	dest := body["destination"].(map[string]any)
	if body["status"] != string(payouts.StatusApproved) || body["concept"] != "Cuota 4 - "+andina || dest["accountNumber"] != companyCVU {
		t.Fatalf("payout = %v", body)
	}
	payoutID := uuid.MustParse(body["id"].(string))
	status, replay := e.do(t, call{method: http.MethodPost, path: path, token: u.token, body: pay, headers: headers})
	if status != http.StatusCreated || replay["id"] != body["id"] {
		t.Fatalf("replay = %d %v", status, replay)
	}

	for name, tc := range map[string]struct {
		body   map[string]any
		status int
		code   apperr.Code
	}{
		"unknown installment": {map[string]any{"amount": map[string]any{"minor": 100, "currency": "ARS"}, "installmentNumber": 99}, http.StatusNotFound, apperr.CodeInstallmentNotFound},
		"no USD CVU":          {map[string]any{"amount": map[string]any{"minor": 100, "currency": "USD"}}, http.StatusUnprocessableEntity, apperr.CodeCurrencyMismatch},
		"over the balance":    {map[string]any{"amount": map[string]any{"minor": 5_000_000_00, "currency": "ARS"}}, http.StatusUnprocessableEntity, apperr.CodeInsufficientFunds},
		"float amount":        {map[string]any{"amount": map[string]any{"minor": 10.5, "currency": "ARS"}}, http.StatusUnprocessableEntity, apperr.CodeValidation},
	} {
		h := map[string]string{"Idempotency-Key": "pay-" + ids.New().String(), auth.HeaderStepUp: e.stepUp(t, u)}
		status, got := e.do(t, call{method: http.MethodPost, path: path, token: u.token, body: tc.body, headers: h})
		if status != tc.status || errCode(got) != string(tc.code) {
			t.Errorf("%s: %d %v", name, status, got)
		}
	}

	e.settle(t, orgID, payoutID)
	p := auth.Principal{UserID: u.id, OrgID: orgID, Role: auth.RoleAdmin, KYCStatus: auth.KYCApproved}
	po, err := e.svc.Payouts.Get(t.Context(), p, payoutID)
	if err != nil || po.Status != payouts.StatusSettled {
		t.Fatalf("payout after dispatch = %s, %v", po.Status, err)
	}
	total, _ := po.Total()
	summary, err := e.svc.Accounts.Summary(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range summary.Totals {
		if b.Posted.Currency == money.ARS && b.Posted.Minor != 2_000_000_00-total.Minor-payinFee(t, e, orgID) {
			t.Fatalf("personal ARS posted = %d after paying %d", b.Posted.Minor, total.Minor)
		}
	}
}

// payinFee is what the funding payin cost (the personal plan may charge payins).
func payinFee(t *testing.T, e *env, orgID uuid.UUID) int64 {
	t.Helper()
	var fee int64
	if err := e.pool.QueryRow(t.Context(), `SELECT COALESCE(sum(fee_minor), 0) FROM payins WHERE org_id = $1`, orgID).Scan(&fee); err != nil {
		t.Fatal(err)
	}
	return fee
}

// settle runs what the worker does: the dispatch job and, with the fake PSP (pending until
// completed), the sweep's re-read after the PSP settles it.
func (e *env) settle(t *testing.T, orgID, payoutID uuid.UUID) {
	t.Helper()
	ctx := t.Context()
	if err := e.svc.Payouts.Dispatch(ctx, payoutID); err != nil {
		t.Fatal(err)
	}
	if e.fakePSP == nil {
		return
	}
	var ref string
	if err := e.pool.QueryRow(ctx, `SELECT provider_attempt_id FROM payouts WHERE id = $1 AND org_id = $2`, payoutID, orgID).Scan(&ref); err != nil {
		t.Fatal(err)
	}
	if err := e.fakePSP.Complete(ref, psp.TransferSettled); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Payouts.Dispatch(ctx, payoutID); err != nil {
		t.Fatal(err)
	}
}
