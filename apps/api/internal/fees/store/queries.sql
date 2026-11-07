-- Fee plan queries. An org without a plan uses the default plan.

-- name: InsertFeePlan :exec
INSERT INTO fee_plans (id, name, is_default) VALUES (sqlc.arg(id), sqlc.arg(name), sqlc.arg(is_default));

-- name: UpsertFeePlanRate :exec
INSERT INTO fee_plan_rates (fee_plan_id, currency, payin_bps, payout_fixed_minor, payout_bps)
VALUES (sqlc.arg(fee_plan_id), sqlc.arg(currency), sqlc.arg(payin_bps), sqlc.arg(payout_fixed_minor), sqlc.arg(payout_bps))
ON CONFLICT (fee_plan_id, currency) DO UPDATE
SET payin_bps = EXCLUDED.payin_bps, payout_fixed_minor = EXCLUDED.payout_fixed_minor, payout_bps = EXCLUDED.payout_bps;

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
