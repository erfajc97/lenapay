-- Account queries (org accounts only; system accounts belong to the ledger feature). Every
-- query filters by org_id (T1); lock order: account rows (by id) before ledger balance rows.

-- name: InsertAccount :exec
INSERT INTO accounts (id, org_id, kind, name, erp_project_ref, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(kind), sqlc.arg(name), sqlc.narg(erp_project_ref),
        sqlc.arg(created_at), sqlc.arg(created_at));

-- name: GetAccount :one
SELECT * FROM accounts WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id);

-- name: GetAccountForUpdate :one
SELECT * FROM accounts WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id) FOR UPDATE;

-- name: LockAccountsShared :many
-- Transfers hold the account rows shared (in id order) so an archive cannot slip in between
-- the archived check and the posting.
SELECT * FROM accounts
WHERE org_id = sqlc.arg(org_id) AND id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY id
FOR SHARE;

-- name: GetMainAccount :one
SELECT * FROM accounts WHERE org_id = sqlc.arg(org_id) AND kind IN ('main', 'personal');

-- name: GetAccountByERPRef :one
SELECT * FROM accounts WHERE org_id = sqlc.arg(org_id) AND erp_project_ref = sqlc.arg(erp_project_ref);

-- name: ListAccounts :many
SELECT * FROM accounts
WHERE org_id = sqlc.arg(org_id)
  AND (sqlc.arg(include_archived)::boolean OR archived_at IS NULL)
ORDER BY created_at, id;

-- name: RenameAccount :execrows
UPDATE accounts SET name = sqlc.arg(name), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id);

-- name: SetAccountArchived :execrows
UPDATE accounts SET archived_at = sqlc.narg(archived_at), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id);
