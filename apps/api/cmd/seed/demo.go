package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/accounts"
	"github.com/erfajc97/lenapay/apps/api/internal/app"
	"github.com/erfajc97/lenapay/apps/api/internal/identity"
	"github.com/erfajc97/lenapay/apps/api/internal/kyc"
	"github.com/erfajc97/lenapay/apps/api/internal/movements"
	"github.com/erfajc97/lenapay/apps/api/internal/orgs"
	"github.com/erfajc97/lenapay/apps/api/internal/payers"
	"github.com/erfajc97/lenapay/apps/api/internal/payouts"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/config"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/page"
)

// Demo credentials (development only; documented in the README).
const (
	demoPassword  = "Lenapay2026!" //nolint:gosec // published demo password, refused in production
	demoEmail     = "demo@lenapay.app"
	approverEmail = "approver@lenapay.app"
	personaEmail  = "persona@lenapay.app"
	orgName       = "Desarrollos Del Sur SA"
	orgCUIT       = "30712345678" // 30-71234567-8, the ERP fixture's CUIT (erp-org-001)
	projectA      = "Torre Palermo"
	projectB      = "Barrio Nordelta Lote 12"
)

// Demo payers: name, DNI, email, account (A or B), and whether they also get a USD CVU.
var demoPayers = []struct {
	name, dni, email string
	project          string
	usd              bool
}{
	{"Federico Álvarez", "21013579", "federico.alvarez@example.com", projectA, true},
	{"Carolina Pereyra", "22027158", "carolina.pereyra@example.com", projectB, false},
	{"Martín Gómez", "23456789", "martin.gomez@example.com", projectA, false},
	{"Lucía Romero", "24567890", "lucia.romero@example.com", projectA, true},
	{"Santiago Díaz", "25678901", "", projectB, false},
	{"Valentina Torres", "26789012", "valentina.torres@example.com", projectB, false},
	{"Joaquín Herrera", "27890123", "joaquin.herrera@example.com", projectA, false},
	{"Camila Acosta", "28901234", "camila.acosta@example.com", projectB, true},
}

// Demo payins: payer index, currency and amount (decimal text; never the bind-mock's
// scripted cents .13/.96–.99).
var demoPayins = []struct {
	payer    int
	currency money.Currency
	amount   string
}{
	{0, money.ARS, "1500000.00"},
	{1, money.ARS, "850000.00"},
	{2, money.ARS, "1200000.00"},
	{3, money.ARS, "640000.50"},
	{4, money.ARS, "990000.00"},
	{5, money.ARS, "1750000.00"},
	{6, money.ARS, "430000.00"},
	{7, money.ARS, "2100000.00"},
	{0, money.ARS, "1500000.00"},
	{2, money.ARS, "780000.00"},
	{5, money.ARS, "560000.25"},
	{0, money.USD, "12000.00"},
	{3, money.USD, "8500.00"},
	{7, money.USD, "15000.00"},
	{1, money.ARS, "325000.00"},
}

// Demo payouts created by the approver user: two wait for the demo user's approval, one is
// approved and settled. Destinations are aliases the bind-mock resolves to outside banks.
var demoPayouts = []struct {
	key, to, concept string
	minor            int64
	settle           bool
}{
	{"seed-payout-hormigon-000001", "hormigonera.sur", "Hormigón elaborado - certificado 7", 42_000_000, false},
	{"seed-payout-ascensores-0001", "ascensores.norte", "Anticipo ascensores torre B", 18_500_000, false},
	{"seed-payout-corralon-000001", "corralon.palermo", "Compra corralón semana 44", 9_850_000, true},
}

type seeder struct {
	cfg      config.Config
	s        *app.Services
	http     *http.Client
	demo     uuid.UUID
	approver uuid.UUID
	org      uuid.UUID
}

func (sd *seeder) admin() auth.Principal {
	return auth.Principal{UserID: sd.demo, OrgID: sd.org, Role: auth.RoleAdmin, KYCStatus: auth.KYCApproved, StepUp: true}
}

func (sd *seeder) approverP() auth.Principal {
	return auth.Principal{UserID: sd.approver, OrgID: sd.org, Role: auth.RoleApprover, KYCStatus: auth.KYCApproved, StepUp: true}
}

// run executes every step; each one checks what exists first, so a second run changes nothing.
func (sd *seeder) run(ctx context.Context) error {
	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"users", sd.users},
		{"company org + KYC", sd.company},
		{"approver membership", sd.membership},
		{"project accounts", sd.projects},
		{"payers + CVUs", sd.payers},
		{"payins via bind-mock", sd.payins},
		{"payouts", sd.payouts},
		{"internal transfer", sd.transfer},
		{"reconciliation run", sd.reconciliation},
		{"personal wallet", sd.persona},
	}
	for _, st := range steps {
		start := time.Now()
		if err := st.fn(ctx); err != nil {
			return fmt.Errorf("seed step %q: %w", st.name, err)
		}
		slog.InfoContext(ctx, "seed step done", "step", st.name, "ms", time.Since(start).Milliseconds())
	}
	return nil
}

// user signs up and verifies a user (the fixed demo code), or finds the existing one.
func (sd *seeder) user(ctx context.Context, email string) (uuid.UUID, error) {
	id, err := sd.s.Identity.Signup(ctx, identity.SignupInput{Email: email, Password: config.Secret(demoPassword)})
	if apperr.IsCode(err, apperr.CodeEmailTaken) {
		return sd.s.Identity.UserIDByEmail(ctx, email) // already verified by a previous run
	}
	if err != nil {
		return uuid.Nil, err
	}
	return id, sd.s.Identity.VerifyEmail(ctx, email, sd.cfg.DevFixedEmailCode)
}

func (sd *seeder) users(ctx context.Context) error {
	var err error
	if sd.demo, err = sd.user(ctx, demoEmail); err != nil {
		return err
	}
	sd.approver, err = sd.user(ctx, approverEmail)
	return err
}

// orgNamed finds the user's org with that name (uuid.Nil when none).
func (sd *seeder) orgNamed(ctx context.Context, userID uuid.UUID, name string) (orgs.Organization, bool, error) {
	list, err := sd.s.Orgs.List(ctx, auth.Principal{UserID: userID})
	if err != nil {
		return orgs.Organization{}, false, err
	}
	for _, o := range list {
		if o.LegalName == name {
			return o, true, nil
		}
	}
	return orgs.Organization{}, false, nil
}

func pdf(name string) kyc.DocumentUpload {
	return kyc.DocumentUpload{FileName: name + ".pdf", Content: strings.NewReader("%PDF-1.7 demo " + name)}
}

func (sd *seeder) company(ctx context.Context) error {
	o, found, err := sd.orgNamed(ctx, sd.demo, orgName)
	if err != nil {
		return err
	}
	owner := auth.Principal{UserID: sd.demo}
	if !found {
		cuit := orgCUIT
		if o, err = sd.s.Orgs.Create(ctx, owner, orgs.CreateInput{Kind: orgs.KindCompany, LegalName: orgName, CUIT: &cuit}); err != nil {
			return err
		}
	}
	sd.org = o.ID
	if o.KYCStatus == auth.KYCApproved {
		return nil
	}
	p := auth.Principal{UserID: sd.demo, OrgID: o.ID, Role: auth.RoleAdmin}
	c, err := sd.s.KYC.Get(ctx, p)
	if err != nil {
		return err
	}
	if c.Status == kyc.StatusDraft || c.Status == kyc.StatusRejected {
		if err := sd.fillCompanyKYC(ctx, p); err != nil {
			return err
		}
		if c, err = sd.s.KYC.Submit(ctx, p); err != nil {
			return err
		}
	}
	if c.Status == kyc.StatusSubmitted { // ops decision (what the internal endpoint does)
		_, err = sd.s.KYC.Decide(ctx, c.ID, kyc.Decision{Approved: true})
	}
	return err
}

func (sd *seeder) fillCompanyKYC(ctx context.Context, p auth.Principal) error {
	rep := "20111222"
	_, err := sd.s.KYC.Update(ctx, p, kyc.UpdateInput{Company: &kyc.CompanyData{
		CUIT: orgCUIT, LegalName: orgName,
		LegalAddress:    kyc.Address{Street: "Av. del Libertador", Number: "5800", City: "CABA", Province: "CABA", PostalCode: "C1428", Country: "AR"},
		Representatives: []kyc.Representative{{FullName: "Diego Morales", DNI: rep, Position: "Presidente"}},
	}})
	if err != nil {
		return err
	}
	docs := []kyc.DocumentUpload{pdf(kyc.DocBylaws), pdf(kyc.DocAFIPConstancy)}
	for _, k := range []string{kyc.DocDNIFront, kyc.DocDNIBack, kyc.DocSelfie} {
		d := pdf(k)
		d.RepresentativeDNI = &rep
		docs = append(docs, d)
	}
	for i := range docs {
		docs[i].Kind = strings.TrimSuffix(docs[i].FileName, ".pdf")
		if _, err := sd.s.KYC.UploadDocument(ctx, p, docs[i]); err != nil {
			return err
		}
	}
	return nil
}

func (sd *seeder) membership(ctx context.Context) error {
	_, err := sd.s.Orgs.AddMember(ctx, sd.admin(), orgs.MemberInput{Email: approverEmail, Role: auth.RoleApprover})
	if apperr.IsCode(err, apperr.CodeMemberAlreadyExists) {
		return nil
	}
	return err
}

// accountByName returns the org's account with that name.
func (sd *seeder) accountByName(ctx context.Context, name string) (accounts.Account, bool, error) {
	list, err := sd.s.Accounts.List(ctx, sd.admin(), false)
	if err != nil {
		return accounts.Account{}, false, err
	}
	for _, a := range list {
		if a.Name == name {
			return a, true, nil
		}
	}
	return accounts.Account{}, false, nil
}

func (sd *seeder) projects(ctx context.Context) error {
	for _, name := range []string{projectA, projectB} {
		if _, found, err := sd.accountByName(ctx, name); err != nil || found {
			if err != nil {
				return err
			}
			continue
		}
		if _, err := sd.s.Accounts.Create(ctx, sd.admin(), accounts.CreateInput{Name: name}); err != nil {
			return err
		}
	}
	return nil
}

// payer finds the demo payer by DNI or creates it.
func (sd *seeder) payer(ctx context.Context, i int) (payers.Payer, error) {
	d := demoPayers[i]
	q := d.dni
	found, err := sd.s.Payers.List(ctx, sd.admin(), payers.ListInput{Query: &q, Page: page.Request{Limit: 5}})
	if err != nil {
		return payers.Payer{}, err
	}
	for _, p := range found.Items {
		if p.DocNumber == d.dni {
			return sd.s.Payers.Get(ctx, sd.admin(), p.ID)
		}
	}
	acc, ok, err := sd.accountByName(ctx, d.project)
	if err != nil || !ok {
		return payers.Payer{}, errors.Join(err, fmt.Errorf("account %q missing", d.project))
	}
	in := payers.CreateInput{AccountID: acc.ID, Name: d.name, DocType: payers.DocDNI, DocNumber: d.dni}
	if d.email != "" {
		in.Email = &d.email
	}
	return sd.s.Payers.Create(ctx, sd.admin(), in)
}

func (sd *seeder) payers(ctx context.Context) error {
	for i, d := range demoPayers {
		p, err := sd.payer(ctx, i)
		if err != nil {
			return err
		}
		currencies := []money.Currency{money.ARS}
		if d.usd {
			currencies = append(currencies, money.USD)
		}
		for _, c := range currencies {
			if cvuOf(p, c) != "" {
				continue
			}
			key := fmt.Sprintf("seed-cvu-%s-%s", d.dni, c)
			if _, err := sd.s.Payers.CreateCVU(ctx, sd.admin(), p.ID, payers.CreateCVUInput{Currency: c, IdempotencyKey: key}); err != nil {
				return err
			}
		}
	}
	return nil
}

func cvuOf(p payers.Payer, c money.Currency) string {
	for _, cvu := range p.CVUs {
		if cvu.Currency == c && cvu.Active {
			return cvu.Number
		}
	}
	return ""
}

// count returns how many of the org's movements of a type exist (and are completed when done).
func (sd *seeder) count(ctx context.Context, typ movements.Type, done bool) (int, error) {
	f := movements.Filter{Type: &typ, Page: page.Request{Limit: page.MaxLimit}}
	if done {
		s := movements.StatusCompleted
		f.Status = &s
	}
	pg, err := sd.s.Movements.List(ctx, sd.admin(), f)
	return len(pg.Items), err
}

// payins sends the missing demo transfers through the bind-mock (it signs and delivers the
// webhook; the worker credits through the ledger), then waits until all are credited.
func (sd *seeder) payins(ctx context.Context) error {
	have, err := sd.count(ctx, movements.TypePayin, false)
	if err != nil {
		return err
	}
	for i := have; i < len(demoPayins); i++ {
		in := demoPayins[i]
		p, err := sd.payer(ctx, in.payer)
		if err != nil {
			return err
		}
		cvu := cvuOf(p, in.currency)
		if cvu == "" {
			return fmt.Errorf("payer %s has no %s CVU", p.Name, in.currency)
		}
		body := map[string]string{
			"cvu": cvu, "amount": in.amount, "currency": string(in.currency),
			"originName": p.Name, "originCuit": "20" + demoPayers[in.payer].dni + "1", // every demo DNI has 8 digits
		}
		if err := sd.mock(ctx, "/_mock/incoming-transfer", body); err != nil {
			return err
		}
	}
	return sd.waitFor(ctx, "payins credited", func() (bool, error) {
		n, err := sd.count(ctx, movements.TypePayin, true)
		return n >= len(demoPayins), err
	})
}

func (sd *seeder) mock(ctx context.Context, path string, body any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(sd.cfg.PSPBaseURL, "/")+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := sd.http.Do(req)
	if err != nil {
		return fmt.Errorf("bind-mock %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("bind-mock %s: %d %s", path, res.StatusCode, msg)
	}
	return nil
}

func (sd *seeder) waitFor(ctx context.Context, what string, ok func() (bool, error)) error {
	deadline := time.Now().Add(90 * time.Second)
	for {
		done, err := ok()
		if err != nil || done {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for %s (is the worker running?)", what)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (sd *seeder) payouts(ctx context.Context) error {
	acc, _, err := sd.accountByName(ctx, projectA)
	if err != nil {
		return err
	}
	have, err := sd.count(ctx, movements.TypePayout, false)
	if err != nil || have >= len(demoPayouts) {
		return err
	}
	for _, d := range demoPayouts {
		po, err := sd.s.Payouts.Create(ctx, sd.approverP(), payouts.CreateInput{
			AccountID: acc.ID, Amount: money.New(d.minor, money.ARS), To: d.to, Concept: d.concept, IdempotencyKey: d.key,
		})
		if err != nil {
			return err
		}
		if !d.settle || po.Status != payouts.StatusPendingApproval {
			continue
		}
		if _, err := sd.s.Payouts.Approve(ctx, sd.admin(), po.ID, "seed-approve-"+d.key[len("seed-payout-"):]); err != nil {
			return err
		}
		if err := sd.waitFor(ctx, "payout settled", func() (bool, error) {
			got, err := sd.s.Payouts.Get(ctx, sd.admin(), po.ID)
			return got.Status == payouts.StatusSettled, err
		}); err != nil {
			return err
		}
	}
	return nil
}

func (sd *seeder) transfer(ctx context.Context) error {
	if n, err := sd.count(ctx, movements.TypeInternalTransfer, false); err != nil || n > 0 {
		return err
	}
	from, _, err := sd.accountByName(ctx, projectA)
	if err != nil {
		return err
	}
	to, _, err := sd.accountByName(ctx, projectB)
	if err != nil {
		return err
	}
	_, err = sd.s.Accounts.Transfer(ctx, sd.admin(), accounts.TransferInput{
		FromAccountID: from.ID, ToAccountID: to.ID, Amount: money.New(25_000_000, money.ARS),
		Concept: "Reasignación de fondos de obra", IdempotencyKey: "seed-transfer-palermo-nordelta",
	})
	return err
}

func (sd *seeder) reconciliation(ctx context.Context) error {
	runs, err := sd.s.Reconciliation.ListRuns(ctx, sd.admin(), nil, page.Request{Limit: 1})
	if err != nil || len(runs.Items) > 0 {
		return err
	}
	run, err := sd.s.Reconciliation.RequestOrgRun(ctx, sd.admin(), money.ARS, time.Now().In(sd.cfg.Location()))
	if err != nil {
		return err
	}
	return sd.waitFor(ctx, "reconciliation run", func() (bool, error) {
		pg, err := sd.s.Reconciliation.ListRuns(ctx, sd.admin(), nil, page.Request{Limit: 5})
		for _, r := range pg.Items {
			if r.ID == run.ID {
				return r.FinishedAt != nil, err
			}
		}
		return false, err
	})
}

// persona is the personal-wallet demo: a person approved by the (fake) vendor at submit.
func (sd *seeder) persona(ctx context.Context) error {
	if !sd.cfg.PersonalModeEnabled {
		slog.WarnContext(ctx, "seed: PERSONAL_MODE_ENABLED is off; skipping persona@lenapay.app")
		return nil
	}
	userID, err := sd.user(ctx, personaEmail)
	if err != nil {
		return err
	}
	const name = "Lucía Fernández"
	o, found, err := sd.orgNamed(ctx, userID, name)
	if err != nil {
		return err
	}
	if !found {
		if o, err = sd.s.Orgs.Create(ctx, auth.Principal{UserID: userID}, orgs.CreateInput{Kind: orgs.KindPersonal, LegalName: name}); err != nil {
			return err
		}
	}
	if o.KYCStatus == auth.KYCApproved {
		return nil
	}
	p := auth.Principal{UserID: userID, OrgID: o.ID, Role: auth.RoleAdmin}
	c, err := sd.s.KYC.Get(ctx, p)
	if err != nil || (c.Status != kyc.StatusDraft && c.Status != kyc.StatusRejected) {
		return err
	}
	if _, err := sd.s.KYC.Update(ctx, p, kyc.UpdateInput{Person: &kyc.PersonData{
		FullName: name, DNI: "30123456", BirthDate: time.Date(1988, 3, 14, 0, 0, 0, 0, time.UTC),
		Address: kyc.Address{Street: "Gorriti", Number: "4500", City: "CABA", Province: "CABA", PostalCode: "C1414", Country: "AR"},
	}}); err != nil {
		return err
	}
	for _, k := range []string{kyc.DocDNIFront, kyc.DocDNIBack, kyc.DocSelfie} {
		d := pdf(k)
		d.Kind = k
		if _, err := sd.s.KYC.UploadDocument(ctx, p, d); err != nil {
			return err
		}
	}
	_, err = sd.s.KYC.Submit(ctx, p) // the fake vendor approves a valid person synchronously
	return err
}
