//go:build integration

package erplink_test

import (
	"context"
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
	"lenapay/apps/api/internal/erplink"
	"lenapay/apps/api/internal/erplink/provider/fake"
	"lenapay/apps/api/internal/orgs"
	"lenapay/apps/api/internal/payers"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/audit"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/clock"
	"lenapay/apps/api/internal/platform/config"
	"lenapay/apps/api/internal/platform/db/dbtest"
	"lenapay/apps/api/internal/platform/ids"
)

var pg *dbtest.Postgres

func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }

type fakeOrgs map[uuid.UUID]string

func (f fakeOrgs) OrgCUIT(_ context.Context, orgID uuid.UUID) (string, error) { return f[orgID], nil }

// recordingImporters stand in for accounts.ImportProject and payers.ImportPayer (lanes B, C),
// idempotent by ERP ref like the real ones.
type recordingImporters struct {
	mu       sync.Mutex
	projects map[string]uuid.UUID
	payers   map[string]payers.ImportInput
}

func (r *recordingImporters) ImportProject(_ context.Context, orgID uuid.UUID, name, ref string) (accounts.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.projects[ref]
	if !ok {
		id = ids.New()
		r.projects[ref] = id
	}
	return accounts.Account{ID: id, OrgID: orgID, Kind: accounts.KindProject, Name: name, ERPProjectRef: &ref}, nil
}

func (r *recordingImporters) ImportPayer(_ context.Context, orgID uuid.UUID, in payers.ImportInput) (payers.Payer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.payers[in.ERPRef] = in
	return payers.Payer{ID: ids.New(), OrgID: orgID, AccountID: in.AccountID, Name: in.Name}, nil
}

// recordingInviter stands in for orgs.InviteERPUsers: it invites everyone as mapped.
type recordingInviter struct {
	mu    sync.Mutex
	calls []orgs.ERPInviteInput
}

func (r *recordingInviter) InviteERPUsers(_ context.Context, in orgs.ERPInviteInput) (orgs.ERPInviteResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, in)
	return orgs.ERPInviteResult{Invited: in.Invitees}, nil
}

type env struct {
	pool      *pgxpool.Pool
	svc       erplink.Service
	invites   *recordingInviter
	imports   *recordingImporters
	dds       uuid.UUID // org with the CUIT of Desarrollos Del Sur SA
	unindexed uuid.UUID
	admin     uuid.UUID
}

func setup(t *testing.T) *env {
	t.Helper()
	pool := pg.NewDatabase(t)
	e := &env{pool: pool, invites: &recordingInviter{}, imports: &recordingImporters{projects: map[string]uuid.UUID{}, payers: map[string]payers.ImportInput{}}}
	e.admin = ids.New()
	e.exec(t, `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, 'x')`, e.admin, e.admin.String()+"@test.local")
	e.dds, e.unindexed = ids.New(), ids.New()
	for _, org := range []uuid.UUID{e.dds, e.unindexed} {
		e.exec(t, `INSERT INTO organizations (id, kind, legal_name, kyc_status, created_by) VALUES ($1, 'company', 'ERP SA', 'approved', $2)`, org, e.admin)
	}
	clk := clock.NewFake(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	inserter, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		t.Fatal(err)
	}
	e.svc = erplink.NewService(erplink.Deps{
		Pool: pool, Clock: clk, Audit: audit.NewLogger(clk), Jobs: inserter, Directory: fake.NewDirectory("../../fixtures/erp"),
		Orgs:     fakeOrgs{e.dds: "30-71234567-8", e.unindexed: "30999999990"},
		Accounts: e.imports, Payers: e.imports, Invites: e.invites,
	})
	if err := e.svc.LoadIdentityIndex(t.Context()); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *env) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := e.pool.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func (e *env) principal(org uuid.UUID) auth.Principal {
	return auth.Principal{UserID: e.admin, OrgID: org, Role: auth.RoleAdmin, KYCStatus: auth.KYCApproved}
}

func creds(password string) erplink.Credentials {
	return erplink.Credentials{Username: "admin@desarrollosdelsur.com.ar", Password: config.Secret(password)}
}

// R-ERP-1: the candidate comes from the index by the org's CUIT.
func TestCandidateByOrgCUIT(t *testing.T) {
	e := setup(t)
	c, err := e.svc.Candidate(t.Context(), e.principal(e.dds))
	if err != nil || !c.Found || *c.ERPOrgID != "erp-org-001" || *c.DisplayName != "Desarrollos Del Sur SA" {
		t.Fatalf("candidate = %+v, %v", c, err)
	}
	none, err := e.svc.Candidate(t.Context(), e.principal(e.unindexed))
	if err != nil || none.Found || none.ERPOrgID != nil {
		t.Fatalf("unindexed candidate = %+v, %v", none, err)
	}
	pending := e.principal(e.dds)
	pending.KYCStatus = auth.KYCSubmitted
	if _, err := e.svc.Candidate(t.Context(), pending); !apperr.IsCode(err, apperr.CodeKYCNotApproved) {
		t.Fatalf("KYC pending err = %v", err)
	}
	// A reload is an upsert: running it again changes nothing.
	if err := e.svc.LoadIdentityIndex(t.Context()); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := e.pool.QueryRow(t.Context(), `SELECT count(*) FROM erp_identity_index`).Scan(&n); err != nil || n != 52 {
		t.Fatalf("index rows = %d (%v), want 10 orgs + 42 buyers (the persona demo buyer included)", n, err)
	}
}

// R-ERP-2: linking needs the ERP credentials; one link per org; the import job is queued.
func TestLinkChecksCredentialsAndQueuesTheImport(t *testing.T) {
	e := setup(t)
	_, err := e.svc.Link(t.Context(), e.principal(e.dds), creds("wrong-password"))
	if !apperr.IsCode(err, apperr.CodeERPCredentialsInvalid) {
		t.Fatalf("wrong password err = %v", err)
	}
	_, err = e.svc.Link(t.Context(), e.principal(e.unindexed), creds(fake.DevPassword))
	if !apperr.IsCode(err, apperr.CodeERPNoCandidate) {
		t.Fatalf("unindexed err = %v", err)
	}
	if _, err := e.svc.Get(t.Context(), e.principal(e.dds)); !apperr.IsCode(err, apperr.CodeERPLinkNotFound) {
		t.Fatalf("get before link err = %v", err)
	}

	link, err := e.svc.Link(t.Context(), e.principal(e.dds), creds(fake.DevPassword))
	if err != nil {
		t.Fatal(err)
	}
	if link.ERPOrgID != "erp-org-001" || link.LinkedBy != e.admin || link.ImportStatus != erplink.ImportPending {
		t.Fatalf("link = %+v", link)
	}
	_, err = e.svc.Link(t.Context(), e.principal(e.dds), creds(fake.DevPassword))
	if !apperr.IsCode(err, apperr.CodeERPAlreadyLinked) {
		t.Fatalf("second link err = %v", err)
	}
	var jobs, audits int
	_ = e.pool.QueryRow(t.Context(), `SELECT count(*) FROM river_job WHERE kind = 'erp_import'`).Scan(&jobs)
	_ = e.pool.QueryRow(t.Context(), `SELECT count(*) FROM audit_log WHERE action = 'erp_link.create'`).Scan(&audits)
	if jobs != 1 || audits != 1 {
		t.Fatalf("jobs %d, audits %d", jobs, audits)
	}
}

// R-ERP-3: projects become project accounts and buyers become payers under their project.
func TestImportCopiesTheSnapshotOnce(t *testing.T) {
	e := setup(t)
	if _, err := e.svc.Link(t.Context(), e.principal(e.dds), creds(fake.DevPassword)); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Import(t.Context(), e.dds); err != nil {
		t.Fatal(err)
	}
	link, err := e.svc.Get(t.Context(), e.principal(e.dds))
	if err != nil || link.ImportStatus != erplink.ImportCompleted || link.ImportedAt == nil {
		t.Fatalf("link = %+v, %v", link, err)
	}
	if len(e.imports.projects) != 3 || len(e.imports.payers) != 5 {
		t.Fatalf("imported %d projects, %d payers", len(e.imports.projects), len(e.imports.payers))
	}
	buyer := e.imports.payers["erp-org-001-buyer-004"]
	if buyer.AccountID != e.imports.projects["erp-org-001-prj-pal"] || buyer.DocType != payers.DocCUIT || buyer.DocNumber != "30700036525" {
		t.Fatalf("company buyer = %+v", buyer)
	}
	// ERP users: the admin, two payments operators, a reconciliations reader and a site manager
	// without payments access (desarrollos-del-sur.json), invited on behalf of the linker.
	if len(e.invites.calls) != 1 || e.invites.calls[0].InvitedBy != e.admin || e.invites.calls[0].OrgID != e.dds {
		t.Fatalf("invite calls = %+v", e.invites.calls)
	}
	want := []orgs.ERPInvitee{
		{Email: "admin@desarrollosdelsur.com.ar", Role: auth.RoleAdmin},
		{Email: "tesoreria@desarrollosdelsur.com.ar", Role: auth.RoleOperator},
		{Email: "cuentas@desarrollosdelsur.com.ar", Role: auth.RoleOperator},
		{Email: "contable@desarrollosdelsur.com.ar", Role: auth.RoleViewer},
	}
	if !slices.Equal(e.invites.calls[0].Invitees, want) {
		t.Fatalf("invitees = %+v", e.invites.calls[0].Invitees)
	}
	sum := link.Summary
	if sum == nil || sum.Projects != 3 || sum.Buyers != 5 || sum.UsersInvited != 4 || sum.UsersWithoutPaymentsAccess != 1 ||
		sum.UsersAlreadyMembers != 0 || len(sum.InvitedUsers) != 4 || sum.InvitedUsers[3] != (erplink.InvitedUser{Email: "contable@desarrollosdelsur.com.ar", Role: auth.RoleViewer}) {
		t.Fatalf("summary = %+v", sum)
	}
	e.imports.payers = map[string]payers.ImportInput{}
	if err := e.svc.Import(t.Context(), e.dds); err != nil || len(e.imports.payers) != 0 {
		t.Fatalf("a completed import ran again: %v, %d payers", err, len(e.imports.payers))
	}
}
