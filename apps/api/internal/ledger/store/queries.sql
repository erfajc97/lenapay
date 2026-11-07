-- Ledger queries. The ledger is the only writer of ledger_* tables and holds (P4).
-- Balance rows are locked with FOR UPDATE in ascending id order (one global lock order, MO3).

-- name: UpsertSystemAccount :one
INSERT INTO accounts (id, org_id, kind, system_code, name)
VALUES (sqlc.arg(id), NULL, 'system', sqlc.arg(system_code), sqlc.arg(name))
ON CONFLICT (system_code) DO UPDATE SET name = accounts.name
RETURNING id;

-- name: InsertLedgerAccount :exec
INSERT INTO ledger_accounts (id, account_id, currency)
VALUES (sqlc.arg(id), sqlc.arg(account_id), sqlc.arg(currency))
ON CONFLICT (account_id, currency) DO NOTHING;

-- name: InsertLedgerBalance :exec
INSERT INTO ledger_balances (ledger_account_id, allow_negative)
VALUES (sqlc.arg(ledger_account_id), sqlc.arg(allow_negative))
ON CONFLICT (ledger_account_id) DO NOTHING;

-- name: GetLedgerAccount :one
SELECT * FROM ledger_accounts WHERE account_id = sqlc.arg(account_id) AND currency = sqlc.arg(currency);

-- name: LockLedgerBalances :many
SELECT * FROM ledger_balances
WHERE ledger_account_id = ANY(sqlc.arg(ledger_account_ids)::uuid[])
ORDER BY ledger_account_id
FOR UPDATE;

-- name: ApplyPosting :exec
UPDATE ledger_balances
SET posted_minor = posted_minor + sqlc.arg(amount_minor), updated_at = sqlc.arg(updated_at)
WHERE ledger_account_id = sqlc.arg(ledger_account_id);

-- name: AdjustHeld :exec
UPDATE ledger_balances
SET held_minor = held_minor + sqlc.arg(delta_minor), updated_at = sqlc.arg(updated_at)
WHERE ledger_account_id = sqlc.arg(ledger_account_id);

-- name: InsertLedgerTransaction :exec
INSERT INTO ledger_transactions (id, kind, ref_type, ref_id, org_id, idempotency_key, memo, created_by)
VALUES (sqlc.arg(id), sqlc.arg(kind), sqlc.arg(ref_type), sqlc.arg(ref_id), sqlc.narg(org_id),
        sqlc.narg(idempotency_key), sqlc.narg(memo), sqlc.narg(created_by));

-- name: InsertLedgerPosting :exec
INSERT INTO ledger_postings (id, transaction_id, ledger_account_id, currency, amount_minor)
VALUES (sqlc.arg(id), sqlc.arg(transaction_id), sqlc.arg(ledger_account_id), sqlc.arg(currency), sqlc.arg(amount_minor));

-- name: InsertHold :exec
INSERT INTO holds (id, ledger_account_id, currency, amount_minor, ref_type, ref_id)
VALUES (sqlc.arg(id), sqlc.arg(ledger_account_id), sqlc.arg(currency), sqlc.arg(amount_minor), sqlc.arg(ref_type), sqlc.arg(ref_id));

-- name: GetHoldForUpdate :one
SELECT * FROM holds WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: ReleaseHold :exec
UPDATE holds SET status = 'released', released_at = sqlc.arg(released_at)
WHERE id = sqlc.arg(id) AND status = 'active';

-- name: CaptureHold :exec
UPDATE holds SET status = 'captured', captured_at = sqlc.arg(captured_at)
WHERE id = sqlc.arg(id) AND status = 'active';

-- name: ListBalancesForAccounts :many
SELECT la.account_id, la.currency, b.posted_minor, b.held_minor
FROM ledger_accounts la
JOIN ledger_balances b ON b.ledger_account_id = la.id
WHERE la.account_id = ANY(sqlc.arg(account_ids)::uuid[])
ORDER BY la.account_id, la.currency;

-- name: ListTransactionPostings :many
SELECT * FROM ledger_postings WHERE transaction_id = sqlc.arg(transaction_id) ORDER BY id;
