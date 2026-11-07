-- Fee plan queries. An org without a plan uses the default plan.

-- name: InsertDefaultFeePlan :exec
-- Concurrent boots race on the single-default index; the loser inserts nothing.
INSERT INTO fee_plans (id, name, is_default)
VALUES (sqlc.arg(id), sqlc.arg(name), true)
ON CONFLICT DO NOTHING;

-- name: InsertFeePlanRateIfMissing :exec
-- Never overwrites a rate an operator changed: boot only fills what is missing.
INSERT INTO fee_plan_rates (fee_plan_id, currency, payin_bps, payout_fixed_minor, payout_bps)
VALUES (sqlc.arg(fee_plan_id), sqlc.arg(currency), sqlc.arg(payin_bps), sqlc.arg(payout_fixed_minor), sqlc.arg(payout_bps))
ON CONFLICT (fee_plan_id, currency) DO NOTHING;

-- name: GetEffectiveFeeRate :one
SELECT r.*
FROM fee_plan_rates r
JOIN fee_plans p ON p.id = r.fee_plan_id
WHERE r.currency = sqlc.arg(currency)
  AND p.id = COALESCE(
        (SELECT o.fee_plan_id FROM organizations o WHERE o.id = sqlc.arg(org_id)),
        (SELECT d.id FROM fee_plans d WHERE d.is_default));

-- name: GetDefaultFeePlan :one
SELECT * FROM fee_plans WHERE is_default;
