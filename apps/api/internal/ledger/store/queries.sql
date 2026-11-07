-- Ledger queries. The ledger is the only writer of ledger_* tables and holds (P4).
-- Balance rows are locked with FOR UPDATE in ascending id order (one global lock order, MO3);
-- a hold row is always locked before the balance rows it touches.

-- name: UpsertSystemAccount :one
INSERT INTO accounts (id, org_id, kind, system_code, name, created_at, updated_at)
VALUES (sqlc.arg(id), NULL, 'system', sqlc.arg(system_code), sqlc.arg(name), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (system_code) DO UPDATE SET name = accounts.name
RETURNING id;

-- name: GetSystemAccountID :one
SELECT id FROM accounts WHERE system_code = sqlc.arg(system_code);

-- name: EnsureLedgerAccount :one
-- Returns the ledger account of (account, currency), creating it when missing.
INSERT INTO ledger_accounts (id, account_id, currency, created_at)
VALUES (sqlc.arg(id), sqlc.arg(account_id), sqlc.arg(currency), sqlc.arg(created_at))
ON CONFLICT (account_id, currency) DO UPDATE SET account_id = ledger_accounts.account_id
RETURNING id;

-- name: InsertLedgerBalance :exec
INSERT INTO ledger_balances (ledger_account_id, allow_negative, updated_at)
VALUES (sqlc.arg(ledger_account_id), sqlc.arg(allow_negative), sqlc.arg(updated_at))
ON CONFLICT (ledger_account_id) DO NOTHING;

-- name: ResolveLedgerAccounts :many
SELECT id, account_id, currency FROM ledger_accounts
WHERE account_id = ANY(sqlc.arg(account_ids)::uuid[]);

-- name: LockLedgerBalances :many
SELECT * FROM ledger_balances
WHERE ledger_account_id = ANY(sqlc.arg(ledger_account_ids)::uuid[])
ORDER BY ledger_account_id
FOR UPDATE;

-- name: LockBalancesForAccounts :many
SELECT la.account_id, la.currency, b.posted_minor, b.held_minor
FROM ledger_balances b
JOIN ledger_accounts la ON la.id = b.ledger_account_id
WHERE la.account_id = ANY(sqlc.arg(account_ids)::uuid[])
ORDER BY b.ledger_account_id
FOR UPDATE OF b;

-- name: ApplyPosting :execrows
-- Conditional: 0 rows means the posting would overdraw a non-system balance.
UPDATE ledger_balances
SET posted_minor = posted_minor + sqlc.arg(amount_minor), updated_at = sqlc.arg(updated_at)
WHERE ledger_account_id = sqlc.arg(ledger_account_id)
  AND (allow_negative OR posted_minor + sqlc.arg(amount_minor) - held_minor >= 0);

-- name: AdjustHeld :execrows
-- Conditional: 0 rows means the hold change would make held negative or exceed the balance.
UPDATE ledger_balances
SET held_minor = held_minor + sqlc.arg(delta_minor), updated_at = sqlc.arg(updated_at)
WHERE ledger_account_id = sqlc.arg(ledger_account_id)
  AND held_minor + sqlc.arg(delta_minor) >= 0
  AND (allow_negative OR posted_minor - (held_minor + sqlc.arg(delta_minor)) >= 0);

-- name: FindTransactionByRef :one
SELECT id FROM ledger_transactions
WHERE ref_type = sqlc.arg(ref_type) AND ref_id = sqlc.arg(ref_id) AND kind = sqlc.arg(kind);

-- name: InsertLedgerTransaction :exec
INSERT INTO ledger_transactions (id, kind, ref_type, ref_id, org_id, idempotency_key, memo, created_by, created_at)
VALUES (sqlc.arg(id), sqlc.arg(kind), sqlc.arg(ref_type), sqlc.arg(ref_id), sqlc.narg(org_id),
        sqlc.narg(idempotency_key), sqlc.narg(memo), sqlc.narg(created_by), sqlc.arg(created_at));

-- name: InsertLedgerPosting :exec
INSERT INTO ledger_postings (id, transaction_id, ledger_account_id, currency, amount_minor, created_at)
VALUES (sqlc.arg(id), sqlc.arg(transaction_id), sqlc.arg(ledger_account_id), sqlc.arg(currency),
        sqlc.arg(amount_minor), sqlc.arg(created_at));

-- name: FindHoldByRef :one
SELECT id FROM holds WHERE ref_type = sqlc.arg(ref_type) AND ref_id = sqlc.arg(ref_id);

-- name: InsertHold :exec
INSERT INTO holds (id, ledger_account_id, currency, amount_minor, ref_type, ref_id, created_at)
VALUES (sqlc.arg(id), sqlc.arg(ledger_account_id), sqlc.arg(currency), sqlc.arg(amount_minor),
        sqlc.arg(ref_type), sqlc.arg(ref_id), sqlc.arg(created_at));

-- name: GetHoldForUpdate :one
SELECT * FROM holds WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: ReleaseHold :execrows
UPDATE holds SET status = 'released', released_at = sqlc.arg(released_at)
WHERE id = sqlc.arg(id) AND status = 'active';

-- name: CaptureHold :execrows
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
