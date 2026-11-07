-- KYC case and document queries. Org-facing reads filter by org_id (T1); the decision path
-- (internal API key, review job) addresses a case by id.

-- name: InsertKYCCase :exec
INSERT INTO kyc_cases (id, org_id, kind, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(kind), sqlc.arg(created_at), sqlc.arg(created_at));

-- name: GetKYCCaseByOrg :one
SELECT * FROM kyc_cases WHERE org_id = sqlc.arg(org_id);

-- name: GetKYCCaseByOrgForUpdate :one
SELECT * FROM kyc_cases WHERE org_id = sqlc.arg(org_id) FOR UPDATE;

-- name: GetKYCCase :one
SELECT * FROM kyc_cases WHERE id = sqlc.arg(id);

-- name: GetKYCCaseForUpdate :one
SELECT * FROM kyc_cases WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: UpdateKYCCaseData :execrows
-- Only a draft or rejected case changes; editing a rejected case reopens it as draft.
UPDATE kyc_cases SET data = sqlc.arg(data), status = 'draft', updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id) AND status IN ('draft', 'rejected');

-- name: SubmitKYCCase :execrows
UPDATE kyc_cases SET status = 'submitted', submitted_at = sqlc.arg(submitted_at), updated_at = sqlc.arg(submitted_at),
    decision_reason = NULL, decided_at = NULL
WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id) AND status IN ('draft', 'rejected');

-- name: DecideKYCCase :execrows
-- draft → submitted → approved | rejected: only a submitted case is decided.
UPDATE kyc_cases
SET status = sqlc.arg(status), decision_reason = sqlc.narg(decision_reason), provider_ref = sqlc.narg(provider_ref),
    decided_at = sqlc.arg(decided_at), updated_at = sqlc.arg(decided_at)
WHERE id = sqlc.arg(id) AND status = 'submitted';

-- name: InsertKYCDocument :exec
INSERT INTO kyc_documents (id, org_id, case_id, kind, representative_dni, file_name, content_type, size_bytes, sha256, storage_key, created_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(case_id), sqlc.arg(kind), sqlc.narg(representative_dni),
        sqlc.arg(file_name), sqlc.arg(content_type), sqlc.arg(size_bytes), sqlc.arg(sha256), sqlc.arg(storage_key),
        sqlc.arg(created_at));

-- name: ListKYCDocuments :many
SELECT * FROM kyc_documents
WHERE case_id = sqlc.arg(case_id) AND org_id = sqlc.arg(org_id)
ORDER BY created_at, id;
