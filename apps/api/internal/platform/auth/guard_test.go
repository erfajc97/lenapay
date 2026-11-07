package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
)

type fakeDirectory struct {
	sessions    map[uuid.UUID]bool // session id → active
	memberships map[[2]uuid.UUID]Membership
}

func (f fakeDirectory) ActiveSession(_ context.Context, _, sessionID uuid.UUID) error {
	if !f.sessions[sessionID] {
		return ErrNotFound
	}
	return nil
}

func (f fakeDirectory) Membership(_ context.Context, orgID, userID uuid.UUID) (Membership, error) {
	m, ok := f.memberships[[2]uuid.UUID{orgID, userID}]
	if !ok {
		return Membership{}, ErrNotFound
	}
	return m, nil
}

type guardFixture struct {
	guard                  *Guard
	issuer                 *Issuer
	user, session, revoked uuid.UUID
	org, foreignOrg        uuid.UUID
	draftOrg               uuid.UUID
}

func newGuardFixture(t *testing.T, role Role) guardFixture {
	t.Helper()
	f := guardFixture{
		issuer: testIssuer(t, clock.Real{}, 7),
		user:   ids.New(), session: ids.New(), revoked: ids.New(),
		org: ids.New(), foreignOrg: ids.New(), draftOrg: ids.New(),
	}
	dir := fakeDirectory{
		sessions: map[uuid.UUID]bool{f.session: true, f.revoked: false},
		memberships: map[[2]uuid.UUID]Membership{
			{f.org, f.user}:      {Role: role, KYCStatus: KYCApproved},
			{f.draftOrg, f.user}: {Role: role, KYCStatus: KYCDraft},
		},
	}
	rules := Rules{
		"publicOp":   Public(),
		"webhookOp":  Signed(),
		"userOp":     Authenticated(),
		"viewOp":     OrgRole(RoleViewer),
		"approveOp":  OrgRole(RoleApprover).WithKYC().WithStepUp(),
		"internalOp": Internal(),
		"brokenOp":   {}, // zero value: must be denied like a missing entry
	}
	f.guard = NewGuard(rules, f.issuer, dir, "internal-key-0123456789")
	return f
}

type call struct {
	op       string
	org      string
	bearer   string
	stepUp   string
	internal string
}

// run sends the call through the guard and returns the error code ("" on success) and the
// principal stored in the context.
func (f guardFixture) run(t *testing.T, c call) (apperr.Code, Principal) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}
	if c.stepUp != "" {
		req.Header.Set(HeaderStepUp, c.stepUp)
	}
	if c.internal != "" {
		req.Header.Set(HeaderInternalKey, c.internal)
	}
	params := map[string]string{}
	if c.org != "" {
		params[OrgPathParam] = c.org
	}
	ctx, err := f.guard.Authorize(t.Context(), req, c.op, params)
	p, _ := PrincipalFrom(ctx)
	return apperr.CodeOf(err), p
}

func TestGuardDecisions(t *testing.T) {
	t.Parallel()
	f := newGuardFixture(t, RoleApprover)
	access, _, _ := f.issuer.IssueAccess(f.user, f.session)
	revoked, _, _ := f.issuer.IssueAccess(f.user, f.revoked)
	stepUp, _, _ := f.issuer.IssueStepUp(f.user, f.session)
	otherSessionStepUp, _, _ := f.issuer.IssueStepUp(f.user, ids.New())
	foreignKey, _, _ := testIssuer(t, clock.Real{}, 9).IssueAccess(f.user, f.session)
	org, foreign, draft := f.org.String(), f.foreignOrg.String(), f.draftOrg.String()

	cases := []struct {
		name string
		call call
		want apperr.Code
	}{
		{"public needs nothing", call{op: "publicOp"}, ""},
		{"signed webhook passes to its handler", call{op: "webhookOp"}, ""},
		{"unknown operation denied", call{op: "notInTable", bearer: access}, apperr.CodeForbidden},
		{"zero-value rule denied", call{op: "brokenOp", bearer: access}, apperr.CodeForbidden},
		{"user route without token", call{op: "userOp"}, apperr.CodeUnauthenticated},
		{"user route with foreign key", call{op: "userOp", bearer: foreignKey}, apperr.CodeUnauthenticated},
		{"user route with step-up as bearer", call{op: "userOp", bearer: stepUp}, apperr.CodeUnauthenticated},
		{"user route with revoked session", call{op: "userOp", bearer: revoked}, apperr.CodeUnauthenticated},
		{"user route ok", call{op: "userOp", bearer: access}, ""},
		{"org route, member", call{op: "viewOp", bearer: access, org: org}, ""},
		// T2: a foreign org answers like a missing one.
		{"org route, not a member", call{op: "viewOp", bearer: access, org: foreign}, apperr.CodeOrgNotFound},
		{"org route, malformed org id", call{op: "viewOp", bearer: access, org: "nope"}, apperr.CodeOrgNotFound},
		{"step-up route without step-up", call{op: "approveOp", bearer: access, org: org}, apperr.CodeStepUpRequired},
		{"step-up of another session", call{op: "approveOp", bearer: access, org: org, stepUp: otherSessionStepUp}, apperr.CodeStepUpRequired},
		{"access token as step-up", call{op: "approveOp", bearer: access, org: org, stepUp: access}, apperr.CodeStepUpRequired},
		{"step-up route ok", call{op: "approveOp", bearer: access, org: org, stepUp: stepUp}, ""},
		{"kyc not approved", call{op: "approveOp", bearer: access, org: draft, stepUp: stepUp}, apperr.CodeKYCNotApproved},
		{"internal without key", call{op: "internalOp"}, apperr.CodeUnauthenticated},
		{"internal wrong key", call{op: "internalOp", internal: "internal-key-0123456780"}, apperr.CodeUnauthenticated},
		{"internal ok", call{op: "internalOp", internal: "internal-key-0123456789"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got, _ := f.run(t, c.call); got != c.want {
				t.Fatalf("code = %q, want %q", got, c.want)
			}
		})
	}
}

// Boundary pair (FT1) on the role check: the minimum role passes, one below is 403.
func TestGuardRoleBoundary(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		role Role
		want apperr.Code
	}{{RoleApprover, ""}, {RoleOperator, apperr.CodeForbidden}, {RoleAdmin, ""}} {
		f := newGuardFixture(t, c.role)
		access, _, _ := f.issuer.IssueAccess(f.user, f.session)
		stepUp, _, _ := f.issuer.IssueStepUp(f.user, f.session)
		if got, _ := f.run(t, call{op: "approveOp", bearer: access, org: f.org.String(), stepUp: stepUp}); got != c.want {
			t.Errorf("role %s: code = %q, want %q", c.role, got, c.want)
		}
	}
}

func TestGuardPassesThePrincipal(t *testing.T) {
	t.Parallel()
	f := newGuardFixture(t, RoleApprover)
	access, _, _ := f.issuer.IssueAccess(f.user, f.session)
	stepUp, _, _ := f.issuer.IssueStepUp(f.user, f.session)
	code, p := f.run(t, call{op: "approveOp", bearer: access, org: f.org.String(), stepUp: stepUp})
	want := Principal{UserID: f.user, SessionID: f.session, OrgID: f.org, Role: RoleApprover, KYCStatus: KYCApproved, StepUp: true}
	if code != "" || p != want {
		t.Fatalf("principal = %+v (code %q), want %+v", p, code, want)
	}
	if _, p := f.run(t, call{op: "userOp", bearer: access}); p.HasOrg() {
		t.Fatalf("user routes carry no org: %+v", p)
	}
}

func TestRequirePrincipal(t *testing.T) {
	t.Parallel()
	if _, err := RequirePrincipal(t.Context()); !apperr.IsCode(err, apperr.CodeUnauthenticated) {
		t.Fatalf("err = %v", err)
	}
	p := Principal{UserID: ids.New()}
	got, err := RequirePrincipal(WithPrincipal(t.Context(), p))
	if err != nil || got != p {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestRuleAccessors(t *testing.T) {
	t.Parallel()
	r := OrgRole(RoleOperator).WithKYC().WithStepUp()
	if !r.IsOrg() || r.MinRole() != RoleOperator || !r.NeedsKYC() || !r.NeedsStepUp() || !r.NeedsToken() || r.IsDeny() {
		t.Fatalf("rule %+v", r)
	}
	if Public().NeedsToken() || Signed().NeedsToken() || Internal().NeedsToken() || !Authenticated().NeedsToken() {
		t.Fatal("token requirement")
	}
	if !(Rule{}).IsDeny() {
		t.Fatal("zero rule must deny")
	}
}
