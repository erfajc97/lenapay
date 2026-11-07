-- Account queries (org accounts only; system accounts belong to the ledger feature).

-- name: InsertAccount :exec
INSERT INTO accounts (id, org_id, kind, name, erp_project_ref)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(kind), sqlc.arg(name), sqlc.narg(erp_project_ref));

-- name: GetAccount :one
SELECT * FROM accounts WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id);

-- name: GetAccountForUpdate :one
SELECT * FROM accounts WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id) FOR UPDATE;

-- name: GetMainAccount :one
SELECT * FROM accounts WHERE org_id = sqlc.arg(org_id) AND kind IN ('main', 'personal');

-- name: ListAccounts :many
SELECT * FROM accounts
WHERE org_id = sqlc.arg(org_id)
  AND (sqlc.arg(include_archived)::boolean OR archived_at IS NULL)
ORDER BY created_at, id;

-- name: RenameAccount :exec
UPDATE accounts SET name = sqlc.arg(name), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id);

-- name: SetAccountArchived :exec
UPDATE accounts SET archived_at = sqlc.narg(archived_at), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id);
