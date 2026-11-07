// Package movements is the read model of what happened to an org's money (lane B, R-MOV-1):
// payins, payouts and internal transfers per org/account, newest first, with keyset
// pagination and filters (type, status, currency, business dates in the configured time
// zone). It only reads: its store queries the ledger tables and pending payouts are merged
// from payouts.Service.
package movements
