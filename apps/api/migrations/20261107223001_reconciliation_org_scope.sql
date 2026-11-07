-- Reconciliation runs are org-scoped (lane D decision): an org run compares the PSP lines that
-- belong to that org with its payins and settled payouts; a run without org is the internal
-- ops run over the whole master account. The daily schedule creates each run at most once.

-- +goose Up
ALTER TABLE reconciliation_runs
    ADD COLUMN org_id uuid REFERENCES organizations (id) ON DELETE RESTRICT;
CREATE INDEX reconciliation_runs_org_created_idx ON reconciliation_runs (org_id, created_at DESC, id DESC);
CREATE UNIQUE INDEX reconciliation_runs_schedule_key
    ON reconciliation_runs (currency, day, COALESCE(org_id, '00000000-0000-0000-0000-000000000000'::uuid))
    WHERE requested_by = 'schedule';

-- +goose Down
DROP INDEX reconciliation_runs_schedule_key;
DROP INDEX reconciliation_runs_org_created_idx;
ALTER TABLE reconciliation_runs DROP COLUMN org_id;
