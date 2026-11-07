// Package movements is the read model of what happened to an org's money (lane B, R-MOV-1):
// payins, payouts and internal-transfer legs per org/account, newest first, with keyset
// pagination and filters (account, type, status, currency, business dates in the configured
// time zone). It only reads: one store query unions the payins and payouts tables with the
// internal-transfer postings of the ledger, so pending payouts (not yet posted) appear too.
package movements
