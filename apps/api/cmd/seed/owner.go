package main

import (
	"context"
	"errors"
	"log/slog"

	"lenapay/apps/api/internal/orgs"
	"lenapay/apps/api/internal/payouts"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/money"
)

// Owner decisions demo (2026-11-07): Desarrollos Del Sur approves company payouts strictly below
// ARS 500.000,00 automatically; the ERP import of Constructora Andina invites its ERP users and
// one of them joins by just signing up.
const (
	policyThresholdMinor = 50_000_000 // ARS 500.000,00
	// The demo payouts above were created before the policy (they wait for 4-eyes); this one,
	// after it, is approved and dispatched at creation.
	belowThresholdKey     = "seed-payout-pintura-0000001"
	belowThresholdMinor   = 12_500_000
	andinaInviteeEmail    = "tesoreria@constructoraandina.com.ar" // operator in the ERP fixture
	andinaInviteeFullName = "Paula Sosa"
)

// errNotJoined: the invitee verified her email but is not an Andina member (the ERP import did
// not invite her, or the auto-accept did not run).
var errNotJoined = errors.New("seed: the ERP invitee did not join Constructora Andina on verification")

// approvalPolicy sets the threshold policy once (re-runs see it in place) and creates the demo
// payout below it, idempotent by its key.
func (sd *seeder) approvalPolicy(ctx context.Context) error {
	current, _, err := sd.s.Orgs.ApprovalPolicy(ctx, sd.owner())
	if err != nil {
		return err
	}
	if current.Mode != orgs.ApprovalThreshold || current.Thresholds[money.ARS] != policyThresholdMinor {
		if _, _, err := sd.s.Orgs.UpdateApprovalPolicy(ctx, sd.owner(), orgs.PolicyInput{
			Mode: orgs.ApprovalThreshold, Thresholds: map[money.Currency]int64{money.ARS: policyThresholdMinor}, AllowOwnerSelfApproval: true,
		}); err != nil {
			return err
		}
	}
	acc, _, err := sd.accountByName(ctx, projectA)
	if err != nil {
		return err
	}
	po, err := sd.s.Payouts.Create(ctx, sd.approverP(), payouts.CreateInput{
		AccountID: acc.ID, Amount: money.New(belowThresholdMinor, money.ARS), To: "pinturas.belgrano",
		Concept: "Pintura hall torre A (bajo el umbral)", IdempotencyKey: belowThresholdKey,
	})
	if err != nil {
		return err
	}
	return sd.waitFor(ctx, "payout below the threshold settled", func() (bool, error) {
		got, err := sd.s.Payouts.Get(ctx, sd.owner(), po.ID)
		return got.Status == payouts.StatusSettled, err
	})
}

// erpInvitee signs up an ERP user the Andina import invited: verifying the email makes her a
// member with her mapped role (auto-accepted invite), with no other step.
func (sd *seeder) erpInvitee(ctx context.Context) error {
	link, err := sd.s.ERPLink.Get(ctx, sd.andina)
	if err != nil {
		return err
	}
	if link.Summary == nil || link.Summary.UsersInvited == 0 {
		// A database seeded before the ERP users import existed: Andina's import already ran
		// without users and never runs again. `make reset` gives the full demo.
		slog.WarnContext(ctx, "seed: Constructora Andina was imported before the ERP users import; skipping the invitee (make reset for the full demo)")
		return nil
	}
	userID, err := sd.user(ctx, andinaInviteeEmail, andinaInviteeFullName)
	if err != nil {
		return err
	}
	list, err := sd.s.Orgs.List(ctx, auth.Principal{UserID: userID})
	if err != nil {
		return err
	}
	for _, o := range list {
		if o.ID == sd.andina.OrgID {
			return nil
		}
	}
	return errNotJoined
}
