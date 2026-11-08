//go:build integration

package orgs_test

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

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
)

var pg *dbtest.Postgres

func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }

// mailbox records invites (the token only travels by email).
type mailbox struct {
	mu     sync.Mutex
	tokens map[string]string
	codes  map[string]string
}

func (m *mailbox) SendInvite(_ context.Context, email, _ string, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[email] = token
	return nil
}

func (m *mailbox) SendEmailCode(_ context.Context, email, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.codes[email] = code
	return nil
}

type env struct {
	pool  *pgxpool.Pool
	clock *clock.Fake
	ids   identity.Service
	orgs  orgs.Service
	mail  *mailbox
}

func newEnv(t *testing.T, personal bool) env {
	t.Helper()
	pool := pg.NewDatabase(t)
	clk := clock.NewFake(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	issuer, err := auth.NewIssuer(auth.IssuerConfig{Issuer: "http://lenapay.test", Seed: make([]byte, ed25519.SeedSize), Clock: clk, AccessTTL: time.Minute, StepUpTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Env: config.EnvTest, RefreshTTL: time.Hour, PersonalModeEnabled: personal}
	box := &mailbox{tokens: map[string]string{}, codes: map[string]string{}}
	e := env{pool: pool, clock: clk, mail: box}
	e.ids = identity.NewService(identity.Deps{
		Pool: pool, Clock: clk, Issuer: issuer, Codes: box, Config: cfg,
		Memberships: identity.MembershipsFunc(func(ctx context.Context, id uuid.UUID) ([]identity.MembershipSummary, error) {
			return e.orgs.MembershipsForUser(ctx, id)
		}),
	})
	var kycSvc kyc.Service
	e.orgs = orgs.NewService(orgs.Deps{
		Pool: pool, Clock: clk, Audit: audit.NewLogger(clk), Users: e.ids, Mailer: box, Config: cfg,
		KYC: orgs.KYCCasesFunc(func(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, kind string) error {
			return kycSvc.OpenCase(ctx, tx, orgID, kind)
		}),
	})
	kycSvc = kyc.NewService(kyc.Deps{
		Pool: pool, Clock: clk, Audit: audit.NewLogger(clk), Provider: kycfake.NewProvider(nil),
		Documents: kycfake.NewDocumentStore(), Orgs: e.orgs,
	})
	return e
}

// user signs up and verifies a user; the principal has no session (orgs never reads it).
func (e env) user(t *testing.T, email string) auth.Principal {
	t.Helper()
	ctx := t.Context()
	id, err := e.ids.Signup(ctx, identity.SignupInput{Email: email, Password: "una-clave-larga-123"})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.ids.VerifyEmail(ctx, email, e.mail.codes[email]); err != nil {
		t.Fatal(err)
	}
	return auth.Principal{UserID: id}
}

// in returns p acting inside org with role (what the guard resolves from memberships).
func in(p auth.Principal, org uuid.UUID, role auth.Role) auth.Principal {
	p.OrgID, p.Role = org, role
	return p
}

func wantCode(t *testing.T, err error, code apperr.Code) {
	t.Helper()
	if !apperr.IsCode(err, code) {
		t.Fatalf("err = %v, want %s", err, code)
	}
}

func TestCreateOrgOpensKYCCaseAndAudits(t *testing.T) {
	t.Parallel()
	e := newEnv(t, false)
	ctx := t.Context()
	ana := e.user(t, "ana@example.com")
	cuit := "30712345678"
	org, err := e.orgs.Create(ctx, ana, orgs.CreateInput{Kind: orgs.KindCompany, LegalName: "  Constructora Sur SA ", CUIT: &cuit})
	if err != nil {
		t.Fatal(err)
	}
	if org.LegalName != "Constructora Sur SA" || org.Role != auth.RoleAdmin || org.KYCStatus != auth.KYCDraft {
		t.Fatalf("org %+v", org)
	}
	var caseKind, caseStatus string
	if err := e.pool.QueryRow(ctx, "SELECT kind, status FROM kyc_cases WHERE org_id = $1", org.ID).Scan(&caseKind, &caseStatus); err != nil {
		t.Fatalf("kyc case not opened: %v", err)
	}
	if caseKind != "company" || caseStatus != "draft" {
		t.Fatalf("case %s/%s", caseKind, caseStatus)
	}
	var audits int
	if err := e.pool.QueryRow(ctx, "SELECT count(*) FROM audit_log WHERE action = 'org.create' AND target_id = $1", org.ID).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("audit rows %d (%v)", audits, err)
	}
	got, err := e.orgs.Get(ctx, in(ana, org.ID, auth.RoleAdmin))
	if err != nil || got.ID != org.ID || *got.CUIT != cuit {
		t.Fatalf("get %+v %v", got, err)
	}
	if c, err := e.orgs.OrgCUIT(ctx, org.ID); err != nil || c != cuit {
		t.Fatalf("OrgCUIT %q %v", c, err)
	}
	_, err = e.orgs.OrgCUIT(ctx, ids.New())
	wantCode(t, err, apperr.CodeOrgNotFound)
}

func TestPersonalOrgs(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	off := newEnv(t, false)
	_, err := off.orgs.Create(ctx, off.user(t, "b@example.com"), orgs.CreateInput{Kind: orgs.KindPersonal, LegalName: "Beto Gómez"})
	wantCode(t, err, apperr.CodePersonalModeDisabled)

	on := newEnv(t, true)
	beto := on.user(t, "b@example.com")
	org, err := on.orgs.Create(ctx, beto, orgs.CreateInput{Kind: orgs.KindPersonal, LegalName: "Beto Gómez"})
	if err != nil {
		t.Fatal(err)
	}
	var caseKind string
	if err := on.pool.QueryRow(ctx, "SELECT kind FROM kyc_cases WHERE org_id = $1", org.ID).Scan(&caseKind); err != nil || caseKind != "person" {
		t.Fatalf("case kind %q (%v)", caseKind, err)
	}
	_, err = on.orgs.Create(ctx, beto, orgs.CreateInput{Kind: orgs.KindPersonal, LegalName: "Beto Gómez"})
	wantCode(t, err, apperr.CodeConflict)
	if _, err := on.orgs.Create(ctx, beto, orgs.CreateInput{Kind: orgs.KindCompany, LegalName: "Beto SRL"}); err != nil {
		t.Fatalf("a person may still create companies: %v", err)
	}
}

// R-ORG-1/2 and T4: invites by email, 7 days, single use, only for the invited email, never
// above the inviter's role.
func TestInvitesAndMembers(t *testing.T) {
	t.Parallel()
	e := newEnv(t, false)
	ctx := t.Context()
	ana := e.user(t, "ana@example.com")
	org, _ := e.orgs.Create(ctx, ana, orgs.CreateInput{Kind: orgs.KindCompany, LegalName: "Constructora Sur"})
	admin := in(ana, org.ID, auth.RoleAdmin)
	carla := e.user(t, "carla@example.com")
	dario := e.user(t, "dario@example.com")

	inv, err := e.orgs.CreateInvite(ctx, admin, orgs.MemberInput{Email: "Carla@Example.com", Role: auth.RoleApprover})
	if err != nil {
		t.Fatal(err)
	}
	if inv.Email != "carla@example.com" || inv.ExpiresAt != e.clock.Now().Add(7*24*time.Hour) {
		t.Fatalf("invite %+v", inv)
	}
	token := e.mail.tokens["carla@example.com"]
	_, err = e.orgs.AcceptInvite(ctx, dario, token) // someone else's invite
	wantCode(t, err, apperr.CodeInviteNotFound)
	m, err := e.orgs.AcceptInvite(ctx, carla, token)
	if err != nil || m.OrgID != org.ID || m.Role != auth.RoleApprover || m.OrgName != "Constructora Sur" {
		t.Fatalf("accept %+v %v", m, err)
	}
	_, err = e.orgs.AcceptInvite(ctx, carla, token) // single use
	wantCode(t, err, apperr.CodeInviteNotFound)
	_, err = e.orgs.AcceptInvite(ctx, carla, "unknown-token-0000000000")
	wantCode(t, err, apperr.CodeInviteNotFound)

	_, err = e.orgs.CreateInvite(ctx, admin, orgs.MemberInput{Email: "carla@example.com", Role: auth.RoleViewer})
	wantCode(t, err, apperr.CodeMemberAlreadyExists)
	_, err = e.orgs.CreateInvite(ctx, in(carla, org.ID, auth.RoleApprover), orgs.MemberInput{Email: "dario@example.com", Role: auth.RoleAdmin})
	wantCode(t, err, apperr.CodeRoleEscalation)

	member, err := e.orgs.AddMember(ctx, admin, orgs.MemberInput{Email: "DARIO@example.com", Role: auth.RoleOperator})
	if err != nil || member.UserID != dario.UserID || member.Email != "dario@example.com" || member.Role != auth.RoleOperator {
		t.Fatalf("add member %+v %v", member, err)
	}
	_, err = e.orgs.AddMember(ctx, admin, orgs.MemberInput{Email: "dario@example.com", Role: auth.RoleViewer})
	wantCode(t, err, apperr.CodeMemberAlreadyExists)
	_, err = e.orgs.AddMember(ctx, admin, orgs.MemberInput{Email: "ghost@example.com", Role: auth.RoleViewer})
	wantCode(t, err, apperr.CodeUserNotFound)

	members, err := e.orgs.ListMembers(ctx, admin)
	if err != nil || len(members) != 3 || members[0].UserID != ana.UserID || members[0].Role != auth.RoleAdmin {
		t.Fatalf("members %+v %v", members, err)
	}
	list, err := e.orgs.List(ctx, carla)
	if err != nil || len(list) != 1 || list[0].Role != auth.RoleApprover {
		t.Fatalf("carla's orgs %+v %v", list, err)
	}
}

func TestInviteExpiry(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		after time.Duration
		want  apperr.Code
	}{
		{"accepted one second before 7 days", 7*24*time.Hour - time.Second, ""},
		{"expired at 7 days", 7 * 24 * time.Hour, apperr.CodeInviteExpired},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			e := newEnv(t, false)
			ana := e.user(t, "ana@example.com")
			eva := e.user(t, "eva@example.com")
			org, _ := e.orgs.Create(ctx, ana, orgs.CreateInput{Kind: orgs.KindCompany, LegalName: "Constructora Sur"})
			if _, err := e.orgs.CreateInvite(ctx, in(ana, org.ID, auth.RoleAdmin), orgs.MemberInput{Email: "eva@example.com", Role: auth.RoleViewer}); err != nil {
				t.Fatal(err)
			}
			e.clock.Advance(tt.after)
			_, err := e.orgs.AcceptInvite(ctx, eva, e.mail.tokens["eva@example.com"])
			if apperr.CodeOf(err) != tt.want {
				t.Fatalf("accept = %v, want %q", err, tt.want)
			}
		})
	}
}

// T2: another tenant's org answers exactly like a missing one; lists never mix tenants.
func TestOtherTenant(t *testing.T) {
	t.Parallel()
	e := newEnv(t, false)
	ctx := t.Context()
	ana := e.user(t, "ana@example.com")
	beto := e.user(t, "beto@example.com")
	orgA, _ := e.orgs.Create(ctx, ana, orgs.CreateInput{Kind: orgs.KindCompany, LegalName: "Constructora Sur"})
	orgB, _ := e.orgs.Create(ctx, beto, orgs.CreateInput{Kind: orgs.KindCompany, LegalName: "Desarrollos Norte"})

	_, err := e.orgs.Get(ctx, in(ana, orgB.ID, auth.RoleAdmin))
	wantCode(t, err, apperr.CodeOrgNotFound)
	listA, _ := e.orgs.List(ctx, ana)
	if len(listA) != 1 || listA[0].ID != orgA.ID {
		t.Fatalf("ana sees %+v", listA)
	}
	members, _ := e.orgs.ListMembers(ctx, in(beto, orgB.ID, auth.RoleAdmin))
	if len(members) != 1 || members[0].UserID != beto.UserID {
		t.Fatalf("org B members %+v", members)
	}
	me, err := e.ids.Me(ctx, ana)
	if err != nil || len(me.Memberships) != 1 || me.Memberships[0].OrgID != orgA.ID {
		t.Fatalf("/me of ana %+v %v", me, err)
	}
}

// Role changes and removals: never above the caller, never losing the last admin, tenant
// scoped (a member of another org is member.not_found), audited.
func TestUpdateAndRemoveMembers(t *testing.T) {
	t.Parallel()
	e := newEnv(t, false)
	ctx := t.Context()
	ana := e.user(t, "ana@example.com")
	org, _ := e.orgs.Create(ctx, ana, orgs.CreateInput{Kind: orgs.KindCompany, LegalName: "Constructora Sur"})
	admin := in(ana, org.ID, auth.RoleAdmin)
	bruno := e.user(t, "bruno@example.com")
	if _, err := e.orgs.AddMember(ctx, admin, orgs.MemberInput{Email: "bruno@example.com", Role: auth.RoleOperator}); err != nil {
		t.Fatal(err)
	}

	// The only admin cannot demote or remove themselves.
	_, err := e.orgs.UpdateMemberRole(ctx, admin, ana.UserID, auth.RoleViewer)
	wantCode(t, err, apperr.CodeLastAdmin)
	wantCode(t, e.orgs.RemoveMember(ctx, admin, ana.UserID), apperr.CodeLastAdmin)

	// An approver cannot grant admin nor touch an admin.
	_, err = e.orgs.UpdateMemberRole(ctx, in(bruno, org.ID, auth.RoleApprover), bruno.UserID, auth.RoleAdmin)
	wantCode(t, err, apperr.CodeRoleEscalation)
	_, err = e.orgs.UpdateMemberRole(ctx, in(bruno, org.ID, auth.RoleApprover), ana.UserID, auth.RoleViewer)
	wantCode(t, err, apperr.CodeRoleEscalation)

	m, err := e.orgs.UpdateMemberRole(ctx, admin, bruno.UserID, auth.RoleAdmin)
	if err != nil || m.Role != auth.RoleAdmin || m.Email != "bruno@example.com" {
		t.Fatalf("promote %+v %v", m, err)
	}
	// Two admins now: ana can step down, then bruno is the last one.
	if _, err := e.orgs.UpdateMemberRole(ctx, admin, ana.UserID, auth.RoleApprover); err != nil {
		t.Fatalf("demote with another admin: %v", err)
	}
	wantCode(t, e.orgs.RemoveMember(ctx, in(bruno, org.ID, auth.RoleAdmin), bruno.UserID), apperr.CodeLastAdmin)
	if err := e.orgs.RemoveMember(ctx, in(bruno, org.ID, auth.RoleAdmin), ana.UserID); err != nil {
		t.Fatalf("remove: %v", err)
	}
	members, _ := e.orgs.ListMembers(ctx, in(bruno, org.ID, auth.RoleAdmin))
	if len(members) != 1 || members[0].UserID != bruno.UserID {
		t.Fatalf("members %+v", members)
	}
	wantCode(t, e.orgs.RemoveMember(ctx, in(bruno, org.ID, auth.RoleAdmin), ana.UserID), apperr.CodeMemberNotFound)

	// Another org's member is not found from here (T).
	other, _ := e.orgs.Create(ctx, ana, orgs.CreateInput{Kind: orgs.KindCompany, LegalName: "Otra SA"})
	_, err = e.orgs.UpdateMemberRole(ctx, in(bruno, org.ID, auth.RoleAdmin), ana.UserID, auth.RoleViewer)
	wantCode(t, err, apperr.CodeMemberNotFound)
	if _, err := e.orgs.Get(ctx, in(ana, other.ID, auth.RoleAdmin)); err != nil {
		t.Fatal(err)
	}
	var audits int
	if err := e.pool.QueryRow(ctx, "SELECT count(*) FROM audit_log WHERE org_id = $1 AND action IN ('member.role_change', 'member.remove')", org.ID).Scan(&audits); err != nil || audits != 3 {
		t.Fatalf("audit rows %d (%v)", audits, err)
	}
}

// Concurrent demotions of the only two admins: exactly one succeeds (the count locks).
func TestConcurrentDemotionsKeepOneAdmin(t *testing.T) {
	t.Parallel()
	e := newEnv(t, false)
	ctx := t.Context()
	ana := e.user(t, "ana@example.com")
	org, _ := e.orgs.Create(ctx, ana, orgs.CreateInput{Kind: orgs.KindCompany, LegalName: "Constructora Sur"})
	bruno := e.user(t, "bruno@example.com")
	if _, err := e.orgs.AddMember(ctx, in(ana, org.ID, auth.RoleAdmin), orgs.MemberInput{Email: "bruno@example.com", Role: auth.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, pair := range [][2]auth.Principal{{ana, bruno}, {bruno, ana}} {
		wg.Go(func() {
			_, errs[i] = e.orgs.UpdateMemberRole(ctx, in(pair[0], org.ID, auth.RoleAdmin), pair[1].UserID, auth.RoleViewer)
		})
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		} else if !apperr.IsCode(err, apperr.CodeLastAdmin) {
			t.Fatalf("unexpected: %v", err)
		}
	}
	var admins int
	if err := e.pool.QueryRow(ctx, "SELECT count(*) FROM memberships WHERE org_id = $1 AND role = 'admin'", org.ID).Scan(&admins); err != nil || ok != 1 || admins != 1 {
		t.Fatalf("ok %d admins %d (%v)", ok, admins, err)
	}
}

// KYC approval copies the verified identity onto the org: the company's CUIT and legal name
// (so the ERP candidate lookup by CUIT works) or the person's DNI and name.
func TestApplyVerifiedIdentity(t *testing.T) {
	t.Parallel()
	e := newEnv(t, false)
	ctx := t.Context()
	ana := e.user(t, "ana@example.com")
	org, _ := e.orgs.Create(ctx, ana, orgs.CreateInput{Kind: orgs.KindCompany, LegalName: "Nombre provisorio"})
	apply := func(id orgs.VerifiedIdentity) error {
		tx, err := e.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if err := e.orgs.ApplyVerifiedIdentity(ctx, tx, org.ID, id); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if err := apply(orgs.VerifiedIdentity{LegalName: " Desarrollos Del Sur SA ", DocType: orgs.DocTypeCUIT, DocNumber: "30712345678"}); err != nil {
		t.Fatal(err)
	}
	got, _ := e.orgs.Get(ctx, in(ana, org.ID, auth.RoleAdmin))
	if got.LegalName != "Desarrollos Del Sur SA" || got.CUIT == nil || *got.CUIT != "30712345678" {
		t.Fatalf("org %+v", got)
	}
	if c, _ := e.orgs.OrgCUIT(ctx, org.ID); c != "30712345678" {
		t.Fatalf("OrgCUIT %q", c)
	}
	// A DNI never lands in the CUIT column (it keeps the last verified CUIT).
	if err := apply(orgs.VerifiedIdentity{LegalName: "Ana Pérez", DocType: orgs.DocTypeDNI, DocNumber: "30111222"}); err != nil {
		t.Fatal(err)
	}
	var docType, docNumber string
	if err := e.pool.QueryRow(ctx, "SELECT doc_type, doc_number FROM organizations WHERE id = $1", org.ID).Scan(&docType, &docNumber); err != nil || docType != "dni" || docNumber != "30111222" {
		t.Fatalf("doc %s/%s (%v)", docType, docNumber, err)
	}
	wantCode(t, apply(orgs.VerifiedIdentity{LegalName: "X", DocType: orgs.DocTypeCUIT, DocNumber: "30712345678"}), apperr.CodeValidation)
	tx, _ := e.pool.Begin(ctx)
	defer func() { _ = tx.Rollback(ctx) }()
	wantCode(t, e.orgs.ApplyVerifiedIdentity(ctx, tx, ids.New(), orgs.VerifiedIdentity{LegalName: "Nadie SA", DocType: orgs.DocTypeCUIT, DocNumber: "30712345678"}), apperr.CodeOrgNotFound)
}

// The signup hook's door: personal mode on → one personal org per user, legal name = the
// email, admin membership and a draft KYC case; personal mode off → nothing.
func TestCreatePersonalForUser(t *testing.T) {
	t.Parallel()
	for _, personal := range []bool{true, false} {
		e := newEnv(t, personal)
		u := e.user(t, fmt.Sprintf("hook-%v@persona.test", personal))
		var orgID uuid.UUID
		var created bool
		err := pgx.BeginFunc(t.Context(), e.pool, func(tx pgx.Tx) error {
			var err error
			orgID, created, err = e.orgs.CreatePersonalForUser(t.Context(), tx, u.UserID, fmt.Sprintf("hook-%v@persona.test", personal))
			return err
		})
		if err != nil || created != personal {
			t.Fatalf("personal=%v: created %v, %v", personal, created, err)
		}
		list, err := e.orgs.List(t.Context(), u)
		if err != nil {
			t.Fatal(err)
		}
		if !personal {
			if len(list) != 0 {
				t.Fatalf("mode off created %v", list)
			}
			continue
		}
		if len(list) != 1 || list[0].ID != orgID || list[0].Kind != orgs.KindPersonal || list[0].Role != auth.RoleAdmin ||
			list[0].KYCStatus != auth.KYCDraft || list[0].LegalName != "hook-true@persona.test" {
			t.Fatalf("personal org = %+v", list)
		}
		var cases int
		if err := e.pool.QueryRow(t.Context(), `SELECT count(*) FROM kyc_cases WHERE org_id = $1 AND status = 'draft'`, orgID).Scan(&cases); err != nil || cases != 1 {
			t.Fatalf("kyc cases = %d, %v", cases, err)
		}
		err = pgx.BeginFunc(t.Context(), e.pool, func(tx pgx.Tx) error {
			_, _, err := e.orgs.CreatePersonalForUser(t.Context(), tx, u.UserID, "otra@persona.test")
			return err
		})
		wantCode(t, err, apperr.CodeConflict)
	}
}
