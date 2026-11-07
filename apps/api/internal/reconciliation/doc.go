// Package reconciliation compares the PSP master-account statement with the ledger, per
// currency and business day (lane D, R-REC-1/2): each movement is matched, missing_in_ledger or
// missing_at_psp, with signed totals (in +, out −) and their difference. Runs are org-scoped:
// an org run keeps only the movements of that org (its payins and settled payouts, plus the
// statement lines whose receiving CVU is the org's); a run without org is the internal ops run
// over the whole master account, requested through the internal endpoint and never listed to
// an org. The hourly reconcile_daily job queues yesterday's runs once (ops + every org with
// ledger activity); reconcile_master executes one run. It owns reconciliation_runs and
// reconciliation_items.
package reconciliation
