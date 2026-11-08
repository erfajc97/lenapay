// Package payerview is "Mis pagos", the B2C bridge (backlog P): a person whose personal org
// passed KYC sees the companies that registered them as a payer, and pays them.
//
// Matching: a company's payer belongs to the person when the payer's document carries the
// person's VERIFIED DNI (the one KYC approval copied onto the personal org): a DNI equal to it,
// or an 11-digit CUIT/CUIL whose middle 8 digits are it. Links are stored in payer_links (this
// feature's only table) on KYC approval (kyc.ApprovalHook, inside the approval transaction) and
// again on every read, so payers created later appear too. Unverified data (a KYC case in draft,
// a company's self-declared CUIT) never matches.
//
// Exposure is minimal: the company's display name, the project the person pays into, the
// person's own CVUs and payins at that company, and (when the company is ERP-linked) the
// person's installments from the ERP. Everything else of the company is read but never returned.
// Payments reuse payouts: a personal payout from the person's personal account to their CVU at
// the company (Idempotency-Key and step-up, sent without approval).
//
// The read side (organizations, accounts, payers, cvus, payins) is a read model like movements:
// queried, never written. The tenant of a link is the personal org, resolved from the session.
package payerview
