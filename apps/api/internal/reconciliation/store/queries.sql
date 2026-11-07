-- Reconciliation run and item queries.

-- name: InsertRun :exec
INSERT INTO reconciliation_runs (id, currency, day, requested_by)
VALUES (sqlc.arg(id), sqlc.arg(currency), sqlc.arg(day), sqlc.arg(requested_by));

-- name: GetRun :one
SELECT * FROM reconciliation_runs WHERE id = sqlc.arg(id);

-- name: ListRuns :many
SELECT * FROM reconciliation_runs
WHERE (sqlc.narg(currency)::char(3) IS NULL OR currency = sqlc.narg(currency)::char(3))
  AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: StartRun :execrows
UPDATE reconciliation_runs SET status = 'running', started_at = sqlc.arg(started_at)
WHERE id = sqlc.arg(id) AND status = 'queued';

-- name: FinishRun :exec
UPDATE reconciliation_runs
SET status = sqlc.arg(status), matched_count = sqlc.arg(matched_count),
    missing_in_ledger_count = sqlc.arg(missing_in_ledger_count), missing_at_psp_count = sqlc.arg(missing_at_psp_count),
    psp_total_minor = sqlc.arg(psp_total_minor), ledger_total_minor = sqlc.arg(ledger_total_minor),
    difference_minor = sqlc.arg(difference_minor), finished_at = sqlc.arg(finished_at)
WHERE id = sqlc.arg(id);

-- name: InsertItem :exec
INSERT INTO reconciliation_items (id, run_id, org_id, provider_transfer_id, direction, currency,
                                  psp_amount_minor, ledger_amount_minor, status)
VALUES (sqlc.arg(id), sqlc.arg(run_id), sqlc.narg(org_id), sqlc.arg(provider_transfer_id), sqlc.arg(direction),
        sqlc.arg(currency), sqlc.narg(psp_amount_minor), sqlc.narg(ledger_amount_minor), sqlc.arg(status));

-- name: ListItems :many
SELECT * FROM reconciliation_items WHERE run_id = sqlc.arg(run_id) ORDER BY created_at, id;
