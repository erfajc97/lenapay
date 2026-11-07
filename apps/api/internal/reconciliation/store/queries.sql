-- Reconciliation run and item queries. A run with org_id is that org's view; without org it is
-- the internal ops run over the whole master account (never listed to an org).

-- name: InsertRun :exec
INSERT INTO reconciliation_runs (id, org_id, currency, day, requested_by, created_at)
VALUES (sqlc.arg(id), sqlc.narg(org_id), sqlc.arg(currency), sqlc.arg(day), sqlc.arg(requested_by), sqlc.arg(created_at));

-- name: InsertScheduledRun :execrows
INSERT INTO reconciliation_runs (id, org_id, currency, day, requested_by, created_at)
VALUES (sqlc.arg(id), sqlc.narg(org_id), sqlc.arg(currency), sqlc.arg(day), 'schedule', sqlc.arg(created_at))
ON CONFLICT DO NOTHING;

-- name: GetRun :one
SELECT * FROM reconciliation_runs WHERE id = sqlc.arg(id);

-- name: ListOrgRuns :many
SELECT * FROM reconciliation_runs
WHERE org_id = sqlc.arg(org_id)
  AND (sqlc.narg(currency)::char(3) IS NULL OR currency = sqlc.narg(currency)::char(3))
  AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: StartRun :execrows
UPDATE reconciliation_runs SET status = 'running', started_at = sqlc.arg(started_at)
WHERE id = sqlc.arg(id) AND status = 'queued';

-- name: FinishRun :exec
UPDATE reconciliation_runs
SET status = 'completed', matched_count = sqlc.arg(matched_count),
    missing_in_ledger_count = sqlc.arg(missing_in_ledger_count), missing_at_psp_count = sqlc.arg(missing_at_psp_count),
    psp_total_minor = sqlc.arg(psp_total_minor), ledger_total_minor = sqlc.arg(ledger_total_minor),
    difference_minor = sqlc.arg(difference_minor), finished_at = sqlc.arg(finished_at)
WHERE id = sqlc.arg(id);

-- name: InsertItem :exec
INSERT INTO reconciliation_items (id, run_id, org_id, provider_transfer_id, direction, currency,
                                  psp_amount_minor, ledger_amount_minor, status, created_at)
VALUES (sqlc.arg(id), sqlc.arg(run_id), sqlc.narg(org_id), sqlc.arg(provider_transfer_id), sqlc.arg(direction),
        sqlc.arg(currency), sqlc.narg(psp_amount_minor), sqlc.narg(ledger_amount_minor), sqlc.arg(status),
        sqlc.arg(created_at));


-- name: GetOrgRun :one
SELECT * FROM reconciliation_runs WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id);

-- name: ListRunItemsPage :many
-- Items of one run in processing order, keyset-paginated on (created_at, id) ascending.
SELECT * FROM reconciliation_items
WHERE run_id = sqlc.arg(run_id)
  AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL
       OR (created_at, id) > (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY created_at, id
LIMIT sqlc.arg(page_limit);
