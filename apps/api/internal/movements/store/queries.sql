-- Movements read model: one row per ledger posting on an org account, newest first.
-- Pending payouts (not yet posted) come from the payouts service, not from this query.

-- name: ListAccountEntries :many
SELECT p.id, t.kind, t.ref_type, t.ref_id, t.memo, la.account_id, p.currency, p.amount_minor, p.created_at
FROM ledger_postings p
JOIN ledger_transactions t ON t.id = p.transaction_id
JOIN ledger_accounts la ON la.id = p.ledger_account_id
JOIN accounts a ON a.id = la.account_id
WHERE a.org_id = sqlc.arg(org_id)
  AND (sqlc.narg(account_id)::uuid IS NULL OR la.account_id = sqlc.narg(account_id)::uuid)
  AND (sqlc.narg(kind)::text IS NULL OR t.kind = sqlc.narg(kind)::text)
  AND (sqlc.narg(currency)::char(3) IS NULL OR p.currency = sqlc.narg(currency)::char(3))
  AND (sqlc.narg(from_time)::timestamptz IS NULL OR p.created_at >= sqlc.narg(from_time)::timestamptz)
  AND (sqlc.narg(to_time)::timestamptz IS NULL OR p.created_at < sqlc.narg(to_time)::timestamptz)
  AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL
       OR (p.created_at, p.id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY p.created_at DESC, p.id DESC
LIMIT sqlc.arg(page_limit);
