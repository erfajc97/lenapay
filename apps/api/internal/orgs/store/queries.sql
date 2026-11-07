-- Organization, membership and invite queries. Every org-owned read filters by org_id (T1).

-- name: InsertOrganization :exec
INSERT INTO organizations (id, kind, legal_name, cuit, fee_plan_id, created_by)
VALUES (sqlc.arg(id), sqlc.arg(kind), sqlc.arg(legal_name), sqlc.narg(cuit), sqlc.narg(fee_plan_id), sqlc.arg(created_by));

-- name: GetOrganization :one
SELECT * FROM organizations WHERE id = sqlc.arg(id);

-- name: GetOrganizationForUser :one
SELECT o.*, m.role
FROM organizations o
JOIN memberships m ON m.org_id = o.id
WHERE o.id = sqlc.arg(org_id) AND m.user_id = sqlc.arg(user_id);

-- name: ListOrganizationsForUser :many
SELECT o.*, m.role
FROM organizations o
JOIN memberships m ON m.org_id = o.id
WHERE m.user_id = sqlc.arg(user_id)
ORDER BY o.created_at, o.id;

-- name: SetOrganizationKYCStatus :exec
UPDATE organizations SET kyc_status = sqlc.arg(kyc_status), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: InsertMembership :exec
INSERT INTO memberships (org_id, user_id, role) VALUES (sqlc.arg(org_id), sqlc.arg(user_id), sqlc.arg(role));

-- name: GetMembership :one
SELECT * FROM memberships WHERE org_id = sqlc.arg(org_id) AND user_id = sqlc.arg(user_id);

-- name: ListMembers :many
SELECT m.user_id, u.email, m.role, m.created_at
FROM memberships m
JOIN users u ON u.id = m.user_id
WHERE m.org_id = sqlc.arg(org_id)
ORDER BY m.created_at, m.user_id;

-- name: CountAdminsForUpdate :one
SELECT count(*) FROM (
    SELECT 1 FROM memberships WHERE org_id = sqlc.arg(org_id) AND role = 'admin' FOR UPDATE
) admins;

-- name: UpdateMembershipRole :exec
UPDATE memberships SET role = sqlc.arg(role), updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND user_id = sqlc.arg(user_id);

-- name: InsertInvite :exec
INSERT INTO invites (id, org_id, email, role, token_hash, invited_by, expires_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(email), sqlc.arg(role), sqlc.arg(token_hash), sqlc.arg(invited_by), sqlc.arg(expires_at));

-- name: GetInviteByTokenHashForUpdate :one
SELECT * FROM invites WHERE token_hash = sqlc.arg(token_hash) FOR UPDATE;

-- name: MarkInviteAccepted :exec
UPDATE invites SET accepted_at = sqlc.arg(accepted_at), accepted_by = sqlc.arg(accepted_by)
WHERE id = sqlc.arg(id) AND accepted_at IS NULL;
