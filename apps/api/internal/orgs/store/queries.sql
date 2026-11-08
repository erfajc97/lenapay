-- Organization, membership and invite queries. Every org-owned read filters by org_id (T1).

-- name: InsertOrganization :exec
INSERT INTO organizations (id, kind, legal_name, cuit, fee_plan_id, created_by, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(kind), sqlc.arg(legal_name), sqlc.narg(cuit), sqlc.narg(fee_plan_id), sqlc.arg(created_by),
        sqlc.arg(created_at), sqlc.arg(created_at));

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

-- name: CountPersonalOrgsCreatedBy :one
-- A person has one personal wallet (R-ACC-1, R-CVU-3).
SELECT count(*) FROM organizations WHERE created_by = sqlc.arg(created_by) AND kind = 'personal';

-- name: SetOrganizationKYCStatus :execrows
UPDATE organizations SET kyc_status = sqlc.arg(kyc_status), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: InsertMembership :exec
INSERT INTO memberships (org_id, user_id, role, created_at, updated_at)
VALUES (sqlc.arg(org_id), sqlc.arg(user_id), sqlc.arg(role), sqlc.arg(created_at), sqlc.arg(created_at));

-- name: GetMembership :one
SELECT * FROM memberships WHERE org_id = sqlc.arg(org_id) AND user_id = sqlc.arg(user_id);

-- name: GetMember :one
SELECT m.user_id, u.email, u.full_name, m.role, m.created_at
FROM memberships m
JOIN users u ON u.id = m.user_id
WHERE m.org_id = sqlc.arg(org_id) AND m.user_id = sqlc.arg(user_id);

-- name: ListMembers :many
SELECT m.user_id, u.email, u.full_name, m.role, m.created_at
FROM memberships m
JOIN users u ON u.id = m.user_id
WHERE m.org_id = sqlc.arg(org_id)
ORDER BY m.created_at, m.user_id;

-- name: UpdateMembershipRole :exec
UPDATE memberships SET role = sqlc.arg(role), updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND user_id = sqlc.arg(user_id);

-- name: InsertInvite :exec
INSERT INTO invites (id, org_id, email, role, token_hash, invited_by, expires_at, created_at, source)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(email), sqlc.arg(role), sqlc.arg(token_hash), sqlc.arg(invited_by),
        sqlc.arg(expires_at), sqlc.arg(created_at), sqlc.arg(source));

-- name: GetInviteByTokenHashForUpdate :one
SELECT * FROM invites WHERE token_hash = sqlc.arg(token_hash) FOR UPDATE;

-- name: MarkInviteAccepted :execrows
UPDATE invites SET accepted_at = sqlc.arg(accepted_at), accepted_by = sqlc.arg(accepted_by)
WHERE id = sqlc.arg(id) AND accepted_at IS NULL;

-- name: GetMembershipForUpdate :one
SELECT * FROM memberships WHERE org_id = sqlc.arg(org_id) AND user_id = sqlc.arg(user_id) FOR UPDATE;

-- name: DeleteMembership :execrows
DELETE FROM memberships WHERE org_id = sqlc.arg(org_id) AND user_id = sqlc.arg(user_id);

-- name: ApplyVerifiedIdentity :execrows
-- KYC approval copies the verified identity onto the org (R-KYC-3, R-ERP-1 lookup by CUIT).
UPDATE organizations
SET legal_name = sqlc.arg(legal_name), cuit = COALESCE(sqlc.narg(cuit), cuit),
    doc_type = sqlc.arg(doc_type), doc_number = sqlc.arg(doc_number), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: CountApproversBesidesOwner :one
-- Members other than the owner who can approve payouts (role approver or admin): owner
-- self-approval is effective only while there are none.
SELECT count(*) FROM memberships WHERE org_id = sqlc.arg(org_id) AND role IN ('approver', 'admin');

-- name: GetApprovalPolicy :one
SELECT * FROM approval_policies WHERE org_id = sqlc.arg(org_id);

-- name: UpsertApprovalPolicy :one
INSERT INTO approval_policies (org_id, mode, threshold_ars_minor, threshold_usd_minor, allow_owner_self_approval,
                               updated_by, updated_at)
VALUES (sqlc.arg(org_id), sqlc.arg(mode), sqlc.narg(threshold_ars_minor), sqlc.narg(threshold_usd_minor),
        sqlc.arg(allow_owner_self_approval), sqlc.arg(updated_by), sqlc.arg(updated_at))
ON CONFLICT (org_id) DO UPDATE
SET mode = EXCLUDED.mode, threshold_ars_minor = EXCLUDED.threshold_ars_minor,
    threshold_usd_minor = EXCLUDED.threshold_usd_minor, allow_owner_self_approval = EXCLUDED.allow_owner_self_approval,
    updated_by = EXCLUDED.updated_by, updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: HasPendingInvite :one
-- An unexpired, unaccepted invite of this email to this org (the ERP import never duplicates).
SELECT EXISTS (
    SELECT 1 FROM invites
    WHERE org_id = sqlc.arg(org_id) AND lower(email) = lower(sqlc.arg(email)) AND accepted_at IS NULL
      AND expires_at > sqlc.arg(now)
);

-- name: ListPendingERPInvitesForUpdate :many
-- ERP import invites of an email still acceptable, oldest first (auto-accepted on email
-- verification and login).
SELECT * FROM invites
WHERE lower(email) = lower(sqlc.arg(email)) AND source = 'erp_import' AND accepted_at IS NULL
  AND expires_at > sqlc.arg(now)
ORDER BY created_at, id
FOR UPDATE;
