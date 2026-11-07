// Package payouts sends money out (lane D, R-OUT-1…6, MO2–MO5). Create resolves the
// destination at the PSP, quotes the fee, then in ONE transaction claims the idempotency key,
// holds amount + fee and inserts the payout (pending_approval for companies, approved for
// personal orgs) with the destination and fee recorded at request time. Approval is 4-eyes
// (the creator never approves; the DB enforces it too) and queues dispatch_payout; the job
// calls the PSP with one reference per attempt; settled captures the hold, rejected releases
// it, uncertain stays `sending` until the sweep re-reads it. It owns payouts and payout_events.
package payouts
