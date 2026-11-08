-- ERP identity index and ERP link queries.

-- name: GetIdentity :one
SELECT * FROM erp_identity_index WHERE doc_type = sqlc.arg(doc_type) AND doc_number = sqlc.arg(doc_number);

-- name: UpsertIdentity :exec
INSERT INTO erp_identity_index (doc_type, doc_number, erp_org_id, erp_client_id, display_name, updated_at)
VALUES (sqlc.arg(doc_type), sqlc.arg(doc_number), sqlc.arg(erp_org_id), sqlc.narg(erp_client_id),
        sqlc.arg(display_name), sqlc.arg(updated_at))
ON CONFLICT (doc_type, doc_number) DO UPDATE
SET erp_org_id = EXCLUDED.erp_org_id, erp_client_id = EXCLUDED.erp_client_id,
    display_name = EXCLUDED.display_name, updated_at = EXCLUDED.updated_at;

-- name: InsertErpLink :exec
INSERT INTO erp_links (id, org_id, erp_org_id, display_name, linked_by, linked_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(erp_org_id), sqlc.arg(display_name), sqlc.arg(linked_by),
        sqlc.arg(linked_at));

-- name: GetErpLinkByOrg :one
SELECT * FROM erp_links WHERE org_id = sqlc.arg(org_id);

-- name: SetErpImportStatus :exec
UPDATE erp_links
SET import_status = sqlc.arg(import_status), imported_at = sqlc.narg(imported_at), snapshot = sqlc.arg(snapshot)
WHERE org_id = sqlc.arg(org_id);

-- name: ListLinkedOrgIDs :many
SELECT org_id FROM erp_links WHERE org_id = ANY(sqlc.arg(org_ids)::uuid[]) ORDER BY org_id;
