-- KYC case and document queries.

-- name: InsertKYCCase :exec
INSERT INTO kyc_cases (id, org_id, kind) VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(kind));

-- name: GetKYCCaseByOrg :one
SELECT * FROM kyc_cases WHERE org_id = sqlc.arg(org_id);

-- name: GetKYCCaseByOrgForUpdate :one
SELECT * FROM kyc_cases WHERE org_id = sqlc.arg(org_id) FOR UPDATE;

-- name: GetKYCCaseForUpdate :one
SELECT * FROM kyc_cases WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: UpdateKYCCaseData :exec
UPDATE kyc_cases SET data = sqlc.arg(data), status = 'draft', updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id);

-- name: SubmitKYCCase :exec
UPDATE kyc_cases SET status = 'submitted', submitted_at = sqlc.arg(submitted_at), updated_at = sqlc.arg(submitted_at)
WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id);

-- name: DecideKYCCase :exec
UPDATE kyc_cases
SET status = sqlc.arg(status), decision_reason = sqlc.narg(decision_reason), provider_ref = sqlc.narg(provider_ref),
    decided_at = sqlc.arg(decided_at), updated_at = sqlc.arg(decided_at)
WHERE id = sqlc.arg(id);

-- name: InsertKYCDocument :exec
INSERT INTO kyc_documents (id, org_id, case_id, kind, representative_dni, file_name, content_type, size_bytes, sha256, storage_key)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(case_id), sqlc.arg(kind), sqlc.narg(representative_dni),
        sqlc.arg(file_name), sqlc.arg(content_type), sqlc.arg(size_bytes), sqlc.arg(sha256), sqlc.arg(storage_key));

-- name: ListKYCDocuments :many
SELECT * FROM kyc_documents
WHERE case_id = sqlc.arg(case_id) AND org_id = sqlc.arg(org_id)
ORDER BY created_at, id;
