//go:build integration

package orgs_test

import (
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"lenapay/apps/api/internal/orgs"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/money"
)

// Role changes and removals (owner decision 2026-11-07): never above the caller (T4), admins are
// changed only by the owner, the owner never through these doors, tenant scoped (a member of
// another org is member.not_found), audited.
func TestUpdateAndRemoveMembers(t *testing.T) {
	t.Parallel()
	e := newEnv(t, false)
	ctx := t.Context()
	ana := e.user(t, "ana@example.com")
	org, _ := e.orgs.Create(ctx, ana, orgs.CreateInput{Kind: orgs.KindCompany, LegalName: "Constructora Sur"})
	owner := in(ana, org.ID, auth.RoleOwner)
	bruno := e.user(t, "bruno@example.com")
	carla := e.user(t, "carla@example.com")
	for _, email := range []string{"bruno@example.com", "carla@example.com"} {
		if _, err := e.orgs.AddMember(ctx, owner, orgs.MemberInput{Email: email, Role: auth.RoleAdmin}); err != nil {
			t.Fatal(err)
		}
	}
	dario := e.user(t, "dario@example.com")
	asBruno := in(bruno, org.ID, auth.RoleAdmin)
	if _, err := e.orgs.AddMember(ctx, asBruno, orgs.MemberInput{Email: "dario@example.com", Role: auth.RoleOperator}); err != nil {
		t.Fatalf("an admin adds below admin: %v", err)
	}

	// The owner never changes or leaves through the member doors, not even by themselves.
	_, err := e.orgs.UpdateMemberRole(ctx, owner, ana.UserID, auth.RoleAdmin)
	wantCode(t, err, apperr.CodeLastAdmin)
	wantCode(t, e.orgs.RemoveMember(ctx, owner, ana.UserID), apperr.CodeLastAdmin)
	// An admin touches neither the owner nor another admin (nor themselves).
	_, err = e.orgs.UpdateMemberRole(ctx, asBruno, ana.UserID, auth.RoleViewer)
	wantCode(t, err, apperr.CodeRoleEscalation)
	wantCode(t, e.orgs.RemoveMember(ctx, asBruno, ana.UserID), apperr.CodeRoleEscalation)
	_, err = e.orgs.UpdateMemberRole(ctx, asBruno, carla.UserID, auth.RoleViewer)
	wantCode(t, err, apperr.CodeRoleEscalation)
	wantCode(t, e.orgs.RemoveMember(ctx, asBruno, carla.UserID), apperr.CodeRoleEscalation)
	_, err = e.orgs.UpdateMemberRole(ctx, asBruno, bruno.UserID, auth.RoleViewer)
	wantCode(t, err, apperr.CodeRoleEscalation)
	// owner is never granted through a member door.
	_, err = e.orgs.UpdateMemberRole(ctx, owner, dario.UserID, auth.RoleOwner)
	wantCode(t, err, apperr.CodeValidation)
	_, err = e.orgs.AddMember(ctx, owner, orgs.MemberInput{Email: "eva@example.com", Role: auth.RoleOwner})
	wantCode(t, err, apperr.CodeValidation)
	_, err = e.orgs.CreateInvite(ctx, owner, orgs.MemberInput{Email: "eva@example.com", Role: auth.RoleOwner})
	wantCode(t, err, apperr.CodeValidation)

	// An admin manages the roles below admin.
	m, err := e.orgs.UpdateMemberRole(ctx, asBruno, dario.UserID, auth.RoleApprover)
	if err != nil || m.Role != auth.RoleApprover || m.Email != "dario@example.com" {
		t.Fatalf("admin changes an operator: %+v %v", m, err)
	}
	// An approver can neither grant admin nor touch an admin.
	_, err = e.orgs.UpdateMemberRole(ctx, in(dario, org.ID, auth.RoleApprover), dario.UserID, auth.RoleAdmin)
	wantCode(t, err, apperr.CodeRoleEscalation)
	_, err = e.orgs.UpdateMemberRole(ctx, in(dario, org.ID, auth.RoleApprover), bruno.UserID, auth.RoleViewer)
	wantCode(t, err, apperr.CodeRoleEscalation)

	// Only the owner demotes and removes admins; the owner is always there, so there is no
	// "last admin" any more.
	if _, err := e.orgs.UpdateMemberRole(ctx, owner, carla.UserID, auth.RoleViewer); err != nil {
		t.Fatalf("owner demotes an admin: %v", err)
	}
	if err := e.orgs.RemoveMember(ctx, owner, bruno.UserID); err != nil {
		t.Fatalf("owner removes the last other admin: %v", err)
	}
	wantCode(t, e.orgs.RemoveMember(ctx, owner, bruno.UserID), apperr.CodeMemberNotFound)
	members, _ := e.orgs.ListMembers(ctx, owner)
	if len(members) != 3 || members[0].UserID != ana.UserID || members[0].Role != auth.RoleOwner {
		t.Fatalf("members %+v", members)
	}

	// Another org's member is not found from here (T).
	other, _ := e.orgs.Create(ctx, bruno, orgs.CreateInput{Kind: orgs.KindCompany, LegalName: "Otra SA"})
	_, err = e.orgs.UpdateMemberRole(ctx, owner, bruno.UserID, auth.RoleViewer)
	wantCode(t, err, apperr.CodeMemberNotFound)
	if _, err := e.orgs.Get(ctx, in(bruno, other.ID, auth.RoleOwner)); err != nil {
		t.Fatal(err)
	}
	var audits int
	if err := e.pool.QueryRow(ctx, "SELECT count(*) FROM audit_log WHERE org_id = $1 AND action IN ('member.role_change', 'member.remove')", org.ID).Scan(&audits); err != nil || audits != 3 {
		t.Fatalf("audit rows %d (%v)", audits, err)
	}
}

// Ownership transfer: owner only (re-read under lock) with step-up, never to a stranger or to
// oneself; the previous owner stays as admin; two transfers at once cannot both win; audited.
func TestTransferOwnership(t *testing.T) {
	t.Parallel()
	e := newEnv(t, false)
	ctx := t.Context()
	ana := e.user(t, "ana@example.com")
	org, _ := e.orgs.Create(ctx, ana, orgs.CreateInput{Kind: orgs.KindCompany, LegalName: "Constructora Sur"})
	owner := in(ana, org.ID, auth.RoleOwner)
	owner.StepUp = true
	bruno, carla := e.user(t, "bruno@example.com"), e.user(t, "carla@example.com")
	stranger := e.user(t, "zoe@example.com")
	for _, email := range []string{"bruno@example.com", "carla@example.com"} {
		if _, err := e.orgs.AddMember(ctx, owner, orgs.MemberInput{Email: email, Role: auth.RoleAdmin}); err != nil {
			t.Fatal(err)
		}
	}
	asAdmin := in(bruno, org.ID, auth.RoleAdmin)
	asAdmin.StepUp = true
	_, _, err := e.orgs.TransferOwnership(ctx, asAdmin, bruno.UserID)
	wantCode(t, err, apperr.CodeForbidden)
	_, _, err = e.orgs.TransferOwnership(ctx, in(ana, org.ID, auth.RoleOwner), bruno.UserID)
	wantCode(t, err, apperr.CodeStepUpRequired)
	_, _, err = e.orgs.TransferOwnership(ctx, owner, ana.UserID)
	wantCode(t, err, apperr.CodeValidation)
	_, _, err = e.orgs.TransferOwnership(ctx, owner, stranger.UserID)
	wantCode(t, err, apperr.CodeMemberNotFound)

	// Two transfers at once by the same (now stale) owner principal: exactly one wins.
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, to := range []uuid.UUID{bruno.UserID, carla.UserID} {
		wg.Go(func() { _, _, errs[i] = e.orgs.TransferOwnership(ctx, owner, to) })
	}
	wg.Wait()
	won := 0
	for _, err := range errs {
		switch {
		case err == nil:
			won++
		case !apperr.IsCode(err, apperr.CodeForbidden):
			t.Fatalf("unexpected: %v", err)
		}
	}
	owners := e.count(t, "SELECT count(*) FROM memberships WHERE org_id = $1 AND role = 'owner'", org.ID)
	anaAdmin := e.count(t, "SELECT count(*) FROM memberships WHERE org_id = $1 AND user_id = $2 AND role = 'admin'", org.ID, ana.UserID)
	if won != 1 || owners != 1 || anaAdmin != 1 {
		t.Fatalf("won %d owners %d ana admin %d", won, owners, anaAdmin)
	}
	if n := e.count(t, "SELECT count(*) FROM audit_log WHERE org_id = $1 AND action = 'org.ownership_transfer'", org.ID); n != 1 {
		t.Fatalf("audit rows %d", n)
	}
}

// The database itself keeps exactly one owner per org, whatever the code does.
func TestOneOwnerConstraint(t *testing.T) {
	t.Parallel()
	e := newEnv(t, false)
	ctx := t.Context()
	ana := e.user(t, "ana@example.com")
	org, _ := e.orgs.Create(ctx, ana, orgs.CreateInput{Kind: orgs.KindCompany, LegalName: "Constructora Sur"})
	bruno := e.user(t, "bruno@example.com")
	if _, err := e.orgs.AddMember(ctx, in(ana, org.ID, auth.RoleOwner), orgs.MemberInput{Email: "bruno@example.com", Role: auth.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	for _, attempt := range []struct {
		name, sql string
		user      uuid.UUID
	}{
		{"a second owner", "UPDATE memberships SET role = 'owner' WHERE org_id = $1 AND user_id = $2", bruno.UserID},
		{"an org without owner", "UPDATE memberships SET role = 'admin' WHERE org_id = $1 AND user_id = $2", ana.UserID},
		{"deleting the owner", "DELETE FROM memberships WHERE org_id = $1 AND user_id = $2", ana.UserID},
	} {
		if _, err := e.pool.Exec(ctx, attempt.sql, org.ID, attempt.user); err == nil {
			t.Fatalf("%s was accepted", attempt.name)
		}
	}
	// Swapping in one transaction (demote first) is how a transfer passes both checks.
	err := pgx.BeginFunc(ctx, e.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "UPDATE memberships SET role = 'admin' WHERE org_id = $1 AND user_id = $2", org.ID, ana.UserID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, "UPDATE memberships SET role = 'owner' WHERE org_id = $1 AND user_id = $2", org.ID, bruno.UserID)
		return err
	})
	if err != nil {
		t.Fatalf("swap in one transaction: %v", err)
	}
}

// Approval policy: defaults until set, owner only with step-up (an admin changing it is the
// attack), threshold mode needs a threshold, audited with before/after. The rules payouts asks
// are the ONE place of the decision: strictly below the threshold; owner self-approval only
// while nobody else can approve.
func TestApprovalPolicy(t *testing.T) {
	t.Parallel()
	e := newEnv(t, false)
	ctx := t.Context()
	ana := e.user(t, "ana@example.com")
	org, _ := e.orgs.Create(ctx, ana, orgs.CreateInput{Kind: orgs.KindCompany, LegalName: "Constructora Sur"})
	owner := in(ana, org.ID, auth.RoleOwner)
	owner.StepUp = true

	policy, effective, err := e.orgs.ApprovalPolicy(ctx, in(ana, org.ID, auth.RoleViewer))
	if err != nil || policy.Mode != orgs.ApprovalAlways || len(policy.Thresholds) != 0 || !policy.AllowOwnerSelfApproval || !effective || policy.UpdatedBy != nil {
		t.Fatalf("default policy %+v effective %v (%v)", policy, effective, err)
	}
	admin := in(ana, org.ID, auth.RoleAdmin)
	admin.StepUp = true
	_, _, err = e.orgs.UpdateApprovalPolicy(ctx, admin, orgs.PolicyInput{Mode: orgs.ApprovalAlways})
	wantCode(t, err, apperr.CodeForbidden)
	_, _, err = e.orgs.UpdateApprovalPolicy(ctx, in(ana, org.ID, auth.RoleOwner), orgs.PolicyInput{Mode: orgs.ApprovalAlways})
	wantCode(t, err, apperr.CodeStepUpRequired)
	_, _, err = e.orgs.UpdateApprovalPolicy(ctx, owner, orgs.PolicyInput{Mode: orgs.ApprovalThreshold, Thresholds: map[money.Currency]int64{}})
	wantCode(t, err, apperr.CodeValidation)
	_, _, err = e.orgs.UpdateApprovalPolicy(ctx, owner, orgs.PolicyInput{Mode: orgs.ApprovalThreshold, Thresholds: map[money.Currency]int64{money.ARS: 0}})
	wantCode(t, err, apperr.CodeValidation)
	if n := e.count(t, "SELECT count(*) FROM approval_policies WHERE org_id = $1", org.ID); n != 0 {
		t.Fatalf("a refused change was stored (%d rows)", n)
	}

	policy, effective, err = e.orgs.UpdateApprovalPolicy(ctx, owner, orgs.PolicyInput{
		Mode: orgs.ApprovalThreshold, Thresholds: map[money.Currency]int64{money.ARS: 50_000_000}, AllowOwnerSelfApproval: false,
	})
	if err != nil || policy.Mode != orgs.ApprovalThreshold || policy.Thresholds[money.ARS] != 50_000_000 || policy.AllowOwnerSelfApproval || effective ||
		policy.UpdatedBy == nil || *policy.UpdatedBy != ana.UserID {
		t.Fatalf("updated policy %+v effective %v (%v)", policy, effective, err)
	}
	if n := e.count(t, "SELECT count(*) FROM audit_log WHERE org_id = $1 AND action = 'org.approval_policy_update'", org.ID); n != 1 {
		t.Fatalf("audit rows %d", n)
	}

	rules, err := e.orgs.ApprovalRules(ctx, org.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		amount money.Amount
		auto   bool
	}{
		{money.New(49_999_999, money.ARS), true}, // the last amount that skips approval
		{money.New(50_000_000, money.ARS), false},
		{money.New(1, money.USD), false}, // no USD threshold: always approval
	} {
		if got := rules.AutoApproves(tt.amount); got != tt.auto {
			t.Errorf("AutoApproves(%v) = %v", tt.amount, got)
		}
	}
	if (orgs.ApprovalRules{Policy: orgs.ApprovalPolicy{Mode: orgs.ApprovalAlways, Thresholds: rules.Policy.Thresholds}}).AutoApproves(money.New(1, money.ARS)) {
		t.Fatal("thresholds are ignored while the mode is always")
	}

	// Self-approval is effective only while nobody else can approve.
	if _, _, err := e.orgs.UpdateApprovalPolicy(ctx, owner, orgs.PolicyInput{Mode: orgs.ApprovalAlways, AllowOwnerSelfApproval: true}); err != nil {
		t.Fatal(err)
	}
	e.user(t, "olga@example.com")
	if _, err := e.orgs.AddMember(ctx, owner, orgs.MemberInput{Email: "olga@example.com", Role: auth.RoleOperator}); err != nil {
		t.Fatal(err)
	}
	if rules, _ := e.orgs.ApprovalRules(ctx, org.ID); !rules.OwnerMaySelfApprove() {
		t.Fatalf("an operator is not an approver: %+v", rules)
	}
	e.user(t, "pia@example.com")
	if _, err := e.orgs.AddMember(ctx, owner, orgs.MemberInput{Email: "pia@example.com", Role: auth.RoleApprover}); err != nil {
		t.Fatal(err)
	}
	if rules, _ := e.orgs.ApprovalRules(ctx, org.ID); rules.OwnerMaySelfApprove() || rules.ApproversBesidesOwner != 1 {
		t.Fatalf("another approver exists: %+v", rules)
	}
}

// ERP users import (orgs side): roles capped at the linker (an owner linker caps at admin, never
// owner), members skipped, pending invites not duplicated; the invites of an email are accepted
// at verification, an expired one is not.
func TestERPInvites(t *testing.T) {
	t.Parallel()
	e := newEnv(t, false)
	ctx := t.Context()
	ana := e.user(t, "ana@example.com")
	org, _ := e.orgs.Create(ctx, ana, orgs.CreateInput{Kind: orgs.KindCompany, LegalName: "Constructora Sur"})
	owner := in(ana, org.ID, auth.RoleOwner)
	e.user(t, "bruno@example.com")
	if _, err := e.orgs.AddMember(ctx, owner, orgs.MemberInput{Email: "bruno@example.com", Role: auth.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	carla := e.user(t, "carla@example.com") // registered before the import, not a member
	invitees := []orgs.ERPInvitee{
		{Email: "Bruno@Example.com", Role: auth.RoleOperator}, // already a member
		{Email: "carla@example.com", Role: auth.RoleAdmin},
		{Email: "dora@example.com", Role: auth.RoleOperator},
		{Email: "eli@example.com", Role: auth.RoleViewer},
	}
	res, err := e.orgs.InviteERPUsers(ctx, orgs.ERPInviteInput{OrgID: org.ID, InvitedBy: ana.UserID, Invitees: invitees})
	if err != nil || res.AlreadyMembers != 1 || len(res.Invited) != 3 || res.Invited[0] != (orgs.ERPInvitee{Email: "carla@example.com", Role: auth.RoleAdmin}) {
		t.Fatalf("invite = %+v, %v", res, err)
	}
	// Re-running (River retries) does not duplicate pending invites.
	if _, err := e.orgs.InviteERPUsers(ctx, orgs.ERPInviteInput{OrgID: org.ID, InvitedBy: ana.UserID, Invitees: invitees}); err != nil {
		t.Fatal(err)
	}
	if n := e.count(t, "SELECT count(*) FROM invites WHERE org_id = $1 AND source = 'erp_import'", org.ID); n != 3 {
		t.Fatalf("erp invites %d", n)
	}
	// A linker who is an admin caps the roles at admin; one who left invites nobody.
	other, _ := e.orgs.Create(ctx, carla, orgs.CreateInput{Kind: orgs.KindCompany, LegalName: "Otra SA"})
	if _, err := e.orgs.AddMember(ctx, in(carla, other.ID, auth.RoleOwner), orgs.MemberInput{Email: "bruno@example.com", Role: auth.RoleApprover}); err != nil {
		t.Fatal(err)
	}
	brunoID, _ := e.ids.UserIDByEmail(ctx, "bruno@example.com")
	capped, err := e.orgs.InviteERPUsers(ctx, orgs.ERPInviteInput{OrgID: other.ID, InvitedBy: brunoID, Invitees: []orgs.ERPInvitee{{Email: "fede@example.com", Role: auth.RoleAdmin}}})
	if err != nil || len(capped.Invited) != 1 || capped.Invited[0].Role != auth.RoleApprover {
		t.Fatalf("capped = %+v, %v", capped, err)
	}
	_, err = e.orgs.InviteERPUsers(ctx, orgs.ERPInviteInput{OrgID: other.ID, InvitedBy: ana.UserID, Invitees: invitees})
	wantCode(t, err, apperr.CodeMemberNotFound)

	// dora signs up and verifies: she is a member with her mapped role, nothing else to do.
	dora := e.user(t, "dora@example.com")
	list, err := e.orgs.List(ctx, dora)
	if err != nil || len(list) != 1 || list[0].ID != org.ID || list[0].Role != auth.RoleOperator {
		t.Fatalf("dora's orgs %+v %v", list, err)
	}
	if n := e.count(t, "SELECT count(*) FROM audit_log WHERE org_id = $1 AND action = 'invite.auto_accept'", org.ID); n != 1 {
		t.Fatalf("auto-accept audits %d", n)
	}
	// An expired ERP invite is not accepted.
	e.clock.Advance(orgs.InviteTTL)
	e.user(t, "eli@example.com")
	eliID, _ := e.ids.UserIDByEmail(ctx, "eli@example.com")
	if n := e.count(t, "SELECT count(*) FROM memberships WHERE org_id = $1 AND user_id = $2", org.ID, eliID); n != 0 {
		t.Fatal("an expired invite was accepted")
	}
}

func (e env) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(t.Context(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
