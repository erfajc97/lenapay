-- Movements read model (R-MOV-1): read-only, one row per payin, payout and internal-transfer
-- leg of an org, newest first, keyset-paginated on (created_at, id). It reads the payins and
-- payouts tables of other features without writing them: a read model, never a writer (P4).
-- Every branch filters by org first and uses that table's (org_id, created_at DESC) index;
-- the internal-transfer branch comes first so sqlc types the nullable columns as nullable;
-- counterparty_name is '' when unknown (the service maps it to null).

-- name: ListMovements :many
SELECT m.id, m.type, m.status, m.direction, m.account_id, m.currency, m.amount_minor, m.fee_minor,
       m.counterparty_name, m.counterparty_cuit, m.counterparty_cbu, m.concept, m.ref_id, m.created_at
FROM (
    SELECT p.id,
           'internal_transfer'::text AS type,
           'completed'::text AS status,
           CASE WHEN p.amount_minor < 0 THEN 'out' ELSE 'in' END::text AS direction,
           la.account_id,
           p.currency::text AS currency,
           abs(p.amount_minor)::bigint AS amount_minor,
           NULL::bigint AS fee_minor,
           (SELECT oa.name FROM ledger_postings op
              JOIN ledger_accounts ola ON ola.id = op.ledger_account_id
              JOIN accounts oa ON oa.id = ola.account_id
            WHERE op.transaction_id = t.id AND op.id <> p.id
            ORDER BY op.id LIMIT 1)::text AS counterparty_name,
           NULL::text AS counterparty_cuit,
           NULL::text AS counterparty_cbu,
           t.memo AS concept,
           t.ref_id,
           t.created_at
    FROM ledger_transactions t
    JOIN ledger_postings p ON p.transaction_id = t.id
    JOIN ledger_accounts la ON la.id = p.ledger_account_id
    WHERE t.org_id = sqlc.arg(org_id)
      AND t.kind = 'internal_transfer'
      AND (sqlc.narg(type)::text IS NULL OR sqlc.narg(type)::text = 'internal_transfer')
      AND (sqlc.narg(account_id)::uuid IS NULL OR la.account_id = sqlc.narg(account_id)::uuid)
      AND (sqlc.narg(currency)::text IS NULL OR p.currency = sqlc.narg(currency)::text)
      AND (sqlc.narg(from_time)::timestamptz IS NULL OR t.created_at >= sqlc.narg(from_time)::timestamptz)
      AND (sqlc.narg(to_time)::timestamptz IS NULL OR t.created_at < sqlc.narg(to_time)::timestamptz)
      AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL
           OR (t.created_at, p.id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid))

    UNION ALL

    SELECT pi.id,
           'payin',
           CASE pi.status WHEN 'credited' THEN 'completed' WHEN 'failed' THEN 'failed' ELSE 'pending' END,
           'in',
           pi.account_id,
           pi.currency::text,
           pi.gross_minor,
           pi.fee_minor,
           COALESCE(pi.origin_name, ''),
           pi.origin_cuit,
           pi.origin_cbu,
           NULL,
           pi.id,
           pi.created_at
    FROM payins pi
    WHERE pi.org_id = sqlc.arg(org_id)
      AND (sqlc.narg(type)::text IS NULL OR sqlc.narg(type)::text = 'payin')
      AND (sqlc.narg(account_id)::uuid IS NULL OR pi.account_id = sqlc.narg(account_id)::uuid)
      AND (sqlc.narg(currency)::text IS NULL OR pi.currency = sqlc.narg(currency)::text)
      AND (sqlc.narg(from_time)::timestamptz IS NULL OR pi.created_at >= sqlc.narg(from_time)::timestamptz)
      AND (sqlc.narg(to_time)::timestamptz IS NULL OR pi.created_at < sqlc.narg(to_time)::timestamptz)
      AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL
           OR (pi.created_at, pi.id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid))

    UNION ALL

    SELECT po.id,
           'payout',
           CASE po.status WHEN 'settled' THEN 'completed' WHEN 'failed' THEN 'failed'
                          WHEN 'rejected' THEN 'cancelled' WHEN 'cancelled' THEN 'cancelled' ELSE 'pending' END,
           'out',
           po.account_id,
           po.currency::text,
           po.amount_minor,
           po.fee_minor,
           po.destination_holder_name,
           po.destination_holder_cuit,
           po.destination_account::text,
           po.concept,
           po.id,
           po.created_at
    FROM payouts po
    WHERE po.org_id = sqlc.arg(org_id)
      AND (sqlc.narg(type)::text IS NULL OR sqlc.narg(type)::text = 'payout')
      AND (sqlc.narg(account_id)::uuid IS NULL OR po.account_id = sqlc.narg(account_id)::uuid)
      AND (sqlc.narg(currency)::text IS NULL OR po.currency = sqlc.narg(currency)::text)
      AND (sqlc.narg(from_time)::timestamptz IS NULL OR po.created_at >= sqlc.narg(from_time)::timestamptz)
      AND (sqlc.narg(to_time)::timestamptz IS NULL OR po.created_at < sqlc.narg(to_time)::timestamptz)
      AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL
           OR (po.created_at, po.id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
) m
WHERE (sqlc.narg(status)::text IS NULL OR m.status = sqlc.narg(status)::text)
ORDER BY m.created_at DESC, m.id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetMovement :one
-- One movement by its id (the posting id of an internal-transfer leg, the payin id or the
-- payout id), same columns and mapping as ListMovements; each branch is a primary-key probe.
SELECT m.id, m.type, m.status, m.direction, m.account_id, m.currency, m.amount_minor, m.fee_minor,
       m.counterparty_name, m.counterparty_cuit, m.counterparty_cbu, m.concept, m.ref_id, m.created_at
FROM (
    SELECT p.id,
           'internal_transfer'::text AS type,
           'completed'::text AS status,
           CASE WHEN p.amount_minor < 0 THEN 'out' ELSE 'in' END::text AS direction,
           la.account_id,
           p.currency::text AS currency,
           abs(p.amount_minor)::bigint AS amount_minor,
           NULL::bigint AS fee_minor,
           (SELECT oa.name FROM ledger_postings op
              JOIN ledger_accounts ola ON ola.id = op.ledger_account_id
              JOIN accounts oa ON oa.id = ola.account_id
            WHERE op.transaction_id = t.id AND op.id <> p.id
            ORDER BY op.id LIMIT 1)::text AS counterparty_name,
           NULL::text AS counterparty_cuit,
           NULL::text AS counterparty_cbu,
           t.memo AS concept,
           t.ref_id,
           t.created_at
    FROM ledger_postings p
    JOIN ledger_transactions t ON t.id = p.transaction_id
    JOIN ledger_accounts la ON la.id = p.ledger_account_id
    WHERE p.id = sqlc.arg(id) AND t.org_id = sqlc.arg(org_id) AND t.kind = 'internal_transfer'

    UNION ALL

    SELECT pi.id, 'payin',
           CASE pi.status WHEN 'credited' THEN 'completed' WHEN 'failed' THEN 'failed' ELSE 'pending' END,
           'in', pi.account_id, pi.currency::text, pi.gross_minor, pi.fee_minor,
           COALESCE(pi.origin_name, ''), pi.origin_cuit, pi.origin_cbu, NULL, pi.id, pi.created_at
    FROM payins pi
    WHERE pi.id = sqlc.arg(id) AND pi.org_id = sqlc.arg(org_id)

    UNION ALL

    SELECT po.id, 'payout',
           CASE po.status WHEN 'settled' THEN 'completed' WHEN 'failed' THEN 'failed'
                          WHEN 'rejected' THEN 'cancelled' WHEN 'cancelled' THEN 'cancelled' ELSE 'pending' END,
           'out', po.account_id, po.currency::text, po.amount_minor, po.fee_minor,
           po.destination_holder_name, po.destination_holder_cuit, po.destination_account::text,
           po.concept, po.id, po.created_at
    FROM payouts po
    WHERE po.id = sqlc.arg(id) AND po.org_id = sqlc.arg(org_id)
) m
LIMIT 1;
