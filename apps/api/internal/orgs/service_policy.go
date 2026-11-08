package orgs

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"lenapay/apps/api/internal/orgs/store"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/db"
	"lenapay/apps/api/internal/platform/money"
)

// maxThresholdMinor is the contract's cap (JS-safe integers) and the DB CHECK.
const maxThresholdMinor = 9007199254740991

func (s *service) ApprovalPolicy(ctx context.Context, p auth.Principal) (ApprovalPolicy, bool, error) {
	if !p.HasOrg() {
		return ApprovalPolicy{}, false, apperr.New(apperr.CodeOrgNotFound, nil)
	}
	rules, err := s.ApprovalRules(ctx, p.OrgID)
	if err != nil {
		return ApprovalPolicy{}, false, err
	}
	return rules.Policy, rules.OwnerMaySelfApprove(), nil
}

func (s *service) ApprovalRules(ctx context.Context, orgID uuid.UUID) (ApprovalRules, error) {
	q := store.New(s.deps.Pool)
	policy, err := loadPolicy(ctx, q, orgID)
	if err != nil {
		return ApprovalRules{}, err
	}
	approvers, err := q.CountApproversBesidesOwner(ctx, orgID)
	if err != nil {
		return ApprovalRules{}, err
	}
	return ApprovalRules{Policy: policy, ApproversBesidesOwner: approvers}, nil
}

func loadPolicy(ctx context.Context, q *store.Queries, orgID uuid.UUID) (ApprovalPolicy, error) {
	row, err := q.GetApprovalPolicy(ctx, orgID)
	if db.IsNoRows(err) {
		return DefaultApprovalPolicy(), nil
	}
	if err != nil {
		return ApprovalPolicy{}, err
	}
	return policyFromRow(row), nil
}

func policyFromRow(row store.ApprovalPolicy) ApprovalPolicy {
	thresholds := map[money.Currency]int64{}
	if row.ThresholdArsMinor != nil {
		thresholds[money.ARS] = *row.ThresholdArsMinor
	}
	if row.ThresholdUsdMinor != nil {
		thresholds[money.USD] = *row.ThresholdUsdMinor
	}
	return ApprovalPolicy{
		Mode: ApprovalMode(row.Mode), Thresholds: thresholds, AllowOwnerSelfApproval: row.AllowOwnerSelfApproval,
		UpdatedBy: &row.UpdatedBy, UpdatedAt: &row.UpdatedAt,
	}
}

// validatePolicy: a known mode, thresholds in [1, max] for known currencies, and threshold mode
// with at least one threshold (otherwise it would mean "always" under another name).
func validatePolicy(in PolicyInput) error {
	if in.Mode != ApprovalAlways && in.Mode != ApprovalThreshold {
		return apperr.New(apperr.CodeValidation, map[string]any{"field": "/mode"})
	}
	for c, minor := range in.Thresholds {
		if _, err := money.ParseCurrency(string(c)); err != nil || minor < 1 || minor > maxThresholdMinor {
			return apperr.New(apperr.CodeValidation, map[string]any{"field": "/thresholdMinor/" + string(c)})
		}
	}
	if in.Mode == ApprovalThreshold && len(in.Thresholds) == 0 {
		return apperr.New(apperr.CodeValidation, map[string]any{"field": "/thresholdMinor"})
	}
	return nil
}

func (s *service) UpdateApprovalPolicy(ctx context.Context, p auth.Principal, in PolicyInput) (ApprovalPolicy, bool, error) {
	if err := requireOwner(p); err != nil {
		return ApprovalPolicy{}, false, err
	}
	if err := validatePolicy(in); err != nil {
		return ApprovalPolicy{}, false, err
	}
	err := db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		before, err := loadPolicy(ctx, q, p.OrgID)
		if err != nil {
			return err
		}
		row, err := q.UpsertApprovalPolicy(ctx, store.UpsertApprovalPolicyParams{
			OrgID: p.OrgID, Mode: string(in.Mode), ThresholdArsMinor: thresholdOf(in.Thresholds, money.ARS),
			ThresholdUsdMinor: thresholdOf(in.Thresholds, money.USD), AllowOwnerSelfApproval: in.AllowOwnerSelfApproval,
			UpdatedBy: p.UserID, UpdatedAt: s.now(),
		})
		if err != nil {
			return err
		}
		after := policyFromRow(row)
		return s.record(ctx, tx, p, p.OrgID, "org.approval_policy_update", "organization", p.OrgID, map[string]any{
			"from": policyAudit(before), "to": policyAudit(after),
		})
	})
	if err != nil {
		return ApprovalPolicy{}, false, err
	}
	return s.ApprovalPolicy(ctx, p)
}

func thresholdOf(thresholds map[money.Currency]int64, c money.Currency) *int64 {
	minor, ok := thresholds[c]
	if !ok {
		return nil
	}
	return &minor
}

// policyAudit is the policy as the audit log keeps it (enums and amounts only).
func policyAudit(p ApprovalPolicy) map[string]any {
	return map[string]any{
		"mode": p.Mode, "thresholdMinor": p.Thresholds, "allowOwnerSelfApproval": p.AllowOwnerSelfApproval,
	}
}
