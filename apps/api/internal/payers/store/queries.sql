-- Payer and CVU queries. Every read filters by org_id (T1); pages are keyset (created_at, id).

-- name: InsertPayer :exec
INSERT INTO payers (id, org_id, account_id, name, doc_type, doc_number, email, erp_ref)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(account_id), sqlc.arg(name), sqlc.arg(doc_type),
        sqlc.arg(doc_number), sqlc.narg(email), sqlc.narg(erp_ref));

-- name: GetPayer :one
SELECT * FROM payers WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id);

-- name: ListPayers :many
SELECT * FROM payers
WHERE org_id = sqlc.arg(org_id)
  AND (sqlc.narg(account_id)::uuid IS NULL OR account_id = sqlc.narg(account_id)::uuid)
  AND (sqlc.narg(search)::text IS NULL
       OR name ILIKE '%' || sqlc.narg(search)::text || '%'
       OR doc_number = sqlc.narg(search)::text)
  AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetPayerByERPRef :one
SELECT * FROM payers WHERE org_id = sqlc.arg(org_id) AND erp_ref = sqlc.arg(erp_ref);

-- name: UpdatePayer :execrows
UPDATE payers SET name = sqlc.arg(name), email = sqlc.narg(email), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id);

-- name: InsertCVU :exec
INSERT INTO cvus (id, org_id, account_id, payer_id, currency, cvu, alias, psp_ref)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(account_id), sqlc.narg(payer_id), sqlc.arg(currency),
        sqlc.arg(cvu), sqlc.narg(alias), sqlc.arg(psp_ref));

-- name: GetCVU :one
SELECT * FROM cvus WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id);

-- name: GetCVUByNumber :one
-- System lookup for payins: the receiving CVU decides the org (no tenant filter by design).
SELECT * FROM cvus WHERE cvu = sqlc.arg(cvu);

-- name: GetActiveCVUForPayer :one
SELECT * FROM cvus
WHERE org_id = sqlc.arg(org_id) AND payer_id = sqlc.arg(payer_id) AND currency = sqlc.arg(currency) AND status = 'active';

-- name: ListCVUsForPayers :many
SELECT * FROM cvus
WHERE org_id = sqlc.arg(org_id) AND payer_id = ANY(sqlc.arg(payer_ids)::uuid[])
ORDER BY created_at, id;

-- name: ListCVUsForAccount :many
SELECT * FROM cvus WHERE org_id = sqlc.arg(org_id) AND account_id = sqlc.arg(account_id) ORDER BY created_at, id;

-- name: UpdateCVUAlias :execrows
UPDATE cvus SET alias = sqlc.arg(alias), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id) AND status = 'active';

-- name: DisableCVU :execrows
UPDATE cvus SET status = 'disabled', disabled_at = sqlc.arg(disabled_at), updated_at = sqlc.arg(disabled_at)
WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id) AND status = 'active';
