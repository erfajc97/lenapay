-- Payer view ("Mis pagos") queries. payer_links is this feature's own table; organizations,
-- memberships, accounts, payers, cvus and payins are only READ (a read model, like movements;
-- never a writer, P4). Every link read filters by the personal org, the link's tenant (T1).

-- name: GetPersonForUser :one
-- The caller's verified person: a personal org they created and administer, KYC approved with
-- the DNI that approval copied onto the org (never the unverified KYC case data).
SELECT o.id, COALESCE(o.doc_number, '')::text AS dni
FROM organizations o
JOIN memberships m ON m.org_id = o.id AND m.user_id = sqlc.arg(user_id) AND m.role = 'admin'
WHERE o.created_by = sqlc.arg(user_id) AND o.kind = 'personal' AND o.kyc_status = 'approved'
  AND o.doc_type = 'dni' AND o.doc_number IS NOT NULL
ORDER BY o.created_at, o.id
LIMIT 1;

-- name: GetVerifiedDNI :one
-- The verified DNI of an approved personal org (inside the KYC approval transaction).
SELECT COALESCE(doc_number, '')::text AS dni FROM organizations
WHERE id = sqlc.arg(id) AND kind = 'personal' AND kyc_status = 'approved'
  AND doc_type = 'dni' AND doc_number IS NOT NULL;

-- name: FindUnlinkedMatches :many
-- Company payers whose document carries this 8-digit DNI and are not linked yet. The CASE is
-- the exact expression of payers_person_dni_idx (migration 20261109100000).
SELECT p.id, p.org_id, p.doc_type
FROM payers p
JOIN organizations o ON o.id = p.org_id AND o.kind = 'company'
WHERE (CASE
           WHEN p.doc_type = 'dni' AND char_length(p.doc_number) <= 8 THEN lpad(p.doc_number, 8, '0')
           WHEN p.doc_type IN ('cuit', 'cuil') AND char_length(p.doc_number) = 11 THEN substr(p.doc_number, 3, 8)
       END) = sqlc.arg(dni8)::text
  AND NOT EXISTS (SELECT 1 FROM payer_links l
                  WHERE l.personal_org_id = sqlc.arg(personal_org_id) AND l.payer_id = p.id)
ORDER BY p.created_at, p.id
LIMIT 200;

-- name: InsertPayerLink :exec
INSERT INTO payer_links (id, personal_org_id, payer_id, org_id, matched_doc_type, matched_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(personal_org_id), sqlc.arg(payer_id), sqlc.arg(org_id),
        sqlc.arg(matched_doc_type), sqlc.arg(matched_at), sqlc.arg(matched_at))
ON CONFLICT (personal_org_id, payer_id) DO NOTHING;

-- name: ListLinks :many
SELECT l.id, l.payer_id, l.org_id, l.status, l.matched_at, o.legal_name AS company_name,
       a.kind AS account_kind, a.name AS account_name, p.erp_ref
FROM payer_links l
JOIN payers p ON p.id = l.payer_id AND p.org_id = l.org_id
JOIN organizations o ON o.id = l.org_id
JOIN accounts a ON a.id = p.account_id AND a.org_id = p.org_id
WHERE l.personal_org_id = sqlc.arg(personal_org_id)
  AND (sqlc.arg(include_hidden)::boolean OR l.status = 'active')
ORDER BY l.matched_at DESC, l.id DESC
LIMIT 100;

-- name: GetLink :one
SELECT l.id, l.payer_id, l.org_id, l.status, l.matched_at, o.legal_name AS company_name,
       a.kind AS account_kind, a.name AS account_name, p.erp_ref
FROM payer_links l
JOIN payers p ON p.id = l.payer_id AND p.org_id = l.org_id
JOIN organizations o ON o.id = l.org_id
JOIN accounts a ON a.id = p.account_id AND a.org_id = p.org_id
WHERE l.id = sqlc.arg(id) AND l.personal_org_id = sqlc.arg(personal_org_id);

-- name: SetLinkStatus :execrows
UPDATE payer_links SET status = sqlc.arg(status), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND personal_org_id = sqlc.arg(personal_org_id);

-- name: ListActiveCVUsForPayers :many
SELECT payer_id, currency, cvu, alias FROM cvus
WHERE payer_id = ANY(sqlc.arg(payer_ids)::uuid[]) AND status = 'active'
ORDER BY payer_id, currency, id;

-- name: PayinTotalsForPayers :many
-- Credited payins per payer and currency (the person's own payments to their CVUs).
SELECT payer_id, currency, sum(gross_minor)::bigint AS total_minor, max(created_at)::timestamptz AS last_at
FROM payins
WHERE payer_id = ANY(sqlc.arg(payer_ids)::uuid[]) AND status = 'credited'
GROUP BY payer_id, currency
ORDER BY payer_id, currency;

-- name: ListPayinsForPayer :many
SELECT pi.id, c.cvu, pi.currency, pi.gross_minor, pi.status, pi.created_at, pi.credited_at
FROM payins pi
JOIN cvus c ON c.id = pi.cvu_id
WHERE pi.payer_id = sqlc.arg(payer_id) AND pi.org_id = sqlc.arg(org_id)
  AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL
       OR (pi.created_at, pi.id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY pi.created_at DESC, pi.id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetActivePayerCVU :one
SELECT cvu FROM cvus
WHERE payer_id = sqlc.arg(payer_id) AND org_id = sqlc.arg(org_id) AND currency = sqlc.arg(currency)
  AND status = 'active';

-- name: GetPersonalAccountID :one
SELECT id FROM accounts
WHERE org_id = sqlc.arg(org_id) AND kind = 'personal' AND archived_at IS NULL
ORDER BY created_at, id
LIMIT 1;
