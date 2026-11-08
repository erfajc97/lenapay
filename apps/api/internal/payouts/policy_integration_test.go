//go:build integration

package payouts_test

import (
	"testing"

	"lenapay/apps/api/internal/orgs"
	"lenapay/apps/api/internal/payouts"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/money"
)

// thresholdRules is a threshold policy of ARS 10 000.00 (= amountMinor).
func thresholdRules() orgs.ApprovalRules {
	return orgs.ApprovalRules{Policy: orgs.ApprovalPolicy{
		Mode: orgs.ApprovalThreshold, Thresholds: map[money.Currency]int64{money.ARS: amountMinor},
	}, ApproversBesidesOwner: 1}
}

// Owner decision 2026-11-07: a company payout STRICTLY below its currency's threshold is approved
// and dispatched at creation (still idempotent and held); at the threshold it waits for approval.
func TestThresholdPolicyBoundary(t *testing.T) {
	e := setup(t)
	e.policies.set(thresholdRules())
	e.fund(t, e.account, 5_000_000)

	below, err := e.svc.Create(t.Context(), e.operator(e.org), createInput(e.account, amountMinor-1, key()))
	if err != nil {
		t.Fatal(err)
	}
	if below.Status != payouts.StatusApproved || len(below.Events) != 1 || below.Events[0].Reason == nil || *below.Events[0].Reason != "policy_threshold" {
		t.Fatalf("below the threshold = %+v", below)
	}
	if n := e.jobs(t, "dispatch_payout"); n != 1 {
		t.Fatalf("dispatch jobs = %d, want 1 (dispatched at once)", n)
	}
	at, err := e.svc.Create(t.Context(), e.operator(e.org), createInput(e.account, amountMinor, key()))
	if err != nil {
		t.Fatal(err)
	}
	if at.Status != payouts.StatusPendingApproval || at.Events[0].Reason != nil {
		t.Fatalf("at the threshold = %+v", at)
	}
	if n := e.jobs(t, "dispatch_payout"); n != 1 {
		t.Fatalf("dispatch jobs = %d, the payout at the threshold must wait", n)
	}
	// Both are held (amount + fee each): approval or not, the money is reserved at creation.
	b := e.balance(t, e.account)
	if b.Held.Minor != below.Amount.Minor+below.Fee.Minor+totalMinor {
		t.Fatalf("held %d", b.Held.Minor)
	}
	// Thresholds do not apply in "always" mode.
	rules := thresholdRules()
	rules.Policy.Mode = orgs.ApprovalAlways
	e.policies.set(rules)
	small, err := e.svc.Create(t.Context(), e.operator(e.org), createInput(e.account, 1_000, key()))
	if err != nil || small.Status != payouts.StatusPendingApproval {
		t.Fatalf("always mode = %+v, %v", small, err)
	}
}

// Owner self-approval: allowed only to the owner, only when the policy allows it and no other
// member can approve; audited as payout.self_approval. Everyone else keeps 4-eyes.
func TestOwnerSelfApproval(t *testing.T) {
	e := setup(t)
	e.fund(t, e.account, 5_000_000)
	owner := e.operator(e.org)
	owner.Role = auth.RoleOwner
	create := func() payouts.Payout {
		t.Helper()
		po, err := e.svc.Create(t.Context(), owner, createInput(e.account, amountMinor, key()))
		if err != nil || po.Status != payouts.StatusPendingApproval {
			t.Fatalf("create = %+v, %v", po, err)
		}
		return po
	}
	alone := orgs.ApprovalRules{Policy: orgs.ApprovalPolicy{Mode: orgs.ApprovalAlways, AllowOwnerSelfApproval: true}}

	// Not allowed by the policy, or another approver exists → the creator cannot approve.
	po := create()
	e.policies.set(orgs.ApprovalRules{Policy: orgs.ApprovalPolicy{Mode: orgs.ApprovalAlways}})
	_, err := e.svc.Approve(t.Context(), owner, po.ID, key())
	wantCode(t, err, apperr.CodePayoutSelfApproval)
	withApprover := alone
	withApprover.ApproversBesidesOwner = 1
	e.policies.set(withApprover)
	_, err = e.svc.Approve(t.Context(), owner, po.ID, key())
	wantCode(t, err, apperr.CodePayoutSelfApproval)
	// An admin creator is never the owner, whatever the policy.
	e.policies.set(alone)
	admin := owner
	admin.Role = auth.RoleAdmin
	_, err = e.svc.Approve(t.Context(), admin, po.ID, key())
	wantCode(t, err, apperr.CodePayoutSelfApproval)

	approved, err := e.svc.Approve(t.Context(), owner, po.ID, key())
	if err != nil || approved.Status != payouts.StatusApproved || approved.ApprovedBy == nil || *approved.ApprovedBy != e.creator {
		t.Fatalf("self approval = %+v, %v", approved, err)
	}
	var self, regular int
	if err := e.pool.QueryRow(t.Context(), `SELECT count(*) FILTER (WHERE action = 'payout.self_approval'), count(*) FILTER (WHERE action = 'payout.approve')
		FROM audit_log WHERE target_id = $1`, po.ID).Scan(&self, &regular); err != nil || self != 1 || regular != 0 {
		t.Fatalf("audit self %d regular %d (%v)", self, regular, err)
	}
	// Another member's approval of the owner's payout is the normal 4-eyes path.
	other := create()
	if _, err := e.svc.Approve(t.Context(), e.approverP(), other.ID, key()); err != nil {
		t.Fatal(err)
	}
}
