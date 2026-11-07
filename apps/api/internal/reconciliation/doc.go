// Package reconciliation compares the PSP master-account statement with the ledger, per
// currency and day (lane D, R-REC-1/2): each movement is matched, missing_in_ledger or
// missing_at_psp, with totals and the difference. Runs are queued (daily periodic job, or the
// internal endpoint) and executed by the reconcile_master job. It owns reconciliation_runs
// and reconciliation_items. Open question for lane D: org admins must only see items of their
// own org (reconciliation_items.org_id), never other tenants' totals.
package reconciliation
