-- Payout queries. Status changes are conditional on the current status (checked row count, MO3).

-- name: InsertPayout :exec
INSERT INTO payouts (id, org_id, account_id, currency, amount_minor, fee_minor, fee_plan_id,
                     destination_type, destination_value, destination_account, destination_holder_name,
                     destination_holder_cuit, destination_bank_name, concept, status, created_by, hold_id,
                     idempotency_key)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(account_id), sqlc.arg(currency), sqlc.arg(amount_minor),
        sqlc.arg(fee_minor), sqlc.narg(fee_plan_id), sqlc.arg(destination_type), sqlc.arg(destination_value),
        sqlc.arg(destination_account), sqlc.arg(destination_holder_name), sqlc.arg(destination_holder_cuit),
        sqlc.narg(destination_bank_name), sqlc.arg(concept), sqlc.arg(status), sqlc.arg(created_by),
        sqlc.narg(hold_id), sqlc.arg(idempotency_key));

-- name: GetPayout :one
SELECT * FROM payouts WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id);

-- name: GetPayoutForUpdate :one
SELECT * FROM payouts WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: TransitionPayout :execrows
UPDATE payouts
SET status = sqlc.arg(to_status), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND status = sqlc.arg(from_status);

-- name: ApprovePayout :execrows
UPDATE payouts
SET status = 'approved', approved_by = sqlc.arg(approved_by), approved_at = sqlc.arg(approved_at),
    updated_at = sqlc.arg(approved_at)
WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id) AND status = 'pending_approval';

-- name: RejectPayout :execrows
UPDATE payouts
SET status = 'rejected', rejected_by = sqlc.arg(rejected_by), reject_reason = sqlc.arg(reject_reason),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id) AND status = 'pending_approval';

-- name: StartPayoutAttempt :execrows
UPDATE payouts
SET status = 'sending', attempt_count = attempt_count + 1, provider_attempt_id = sqlc.arg(provider_attempt_id),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND status IN ('approved', 'sending');

-- name: SettlePayout :execrows
UPDATE payouts
SET status = 'settled', provider_transfer_id = sqlc.arg(provider_transfer_id),
    ledger_transaction_id = sqlc.arg(ledger_transaction_id), settled_at = sqlc.arg(settled_at),
    updated_at = sqlc.arg(settled_at)
WHERE id = sqlc.arg(id) AND status = 'sending';

-- name: FailPayout :execrows
UPDATE payouts
SET status = 'failed', failure_code = sqlc.arg(failure_code), failed_at = sqlc.arg(failed_at),
    updated_at = sqlc.arg(failed_at)
WHERE id = sqlc.arg(id) AND status = 'sending';

-- name: InsertPayoutEvent :exec
INSERT INTO payout_events (id, payout_id, org_id, from_status, to_status, actor_id, reason)
VALUES (sqlc.arg(id), sqlc.arg(payout_id), sqlc.arg(org_id), sqlc.narg(from_status), sqlc.arg(to_status),
        sqlc.narg(actor_id), sqlc.narg(reason));

-- name: ListPayoutEvents :many
SELECT * FROM payout_events WHERE payout_id = sqlc.arg(payout_id) AND org_id = sqlc.arg(org_id) ORDER BY created_at, id;

-- name: ListPendingApprovals :many
SELECT * FROM payouts
WHERE org_id = sqlc.arg(org_id) AND status = 'pending_approval'
  AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: ListSendingPayouts :many
SELECT * FROM payouts WHERE status = 'sending' AND updated_at < sqlc.arg(updated_before) ORDER BY updated_at LIMIT sqlc.arg(page_limit);

-- name: ListSettledPayoutsBetween :many
SELECT * FROM payouts
WHERE currency = sqlc.arg(currency) AND status = 'settled'
  AND settled_at >= sqlc.arg(from_time) AND settled_at < sqlc.arg(to_time)
ORDER BY settled_at, id;
