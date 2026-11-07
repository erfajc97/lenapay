// Package ledger is the double-entry ledger and the ONLY writer of balances (lane B, P4,
// R-ACC-2, R-NF-2). Every call that moves money takes the caller's transaction, so the
// posting, the business row and the idempotency record commit together. The database backs
// the invariants: each transaction sums to 0 per currency (deferred trigger), postings are
// append-only, a non-system balance never goes below its holds, and (ref_type, ref_id, kind)
// is unique so the same fact never posts twice (MO3). Balance rows are locked FOR UPDATE in
// ascending ledger-account id order.
package ledger
