// Package psp is the port to the payment service provider (BIND COBRO + CVUCOLLECT, lane C,
// P2, A9, AG7). It holds only the Provider interface and its domain types; the HTTP adapter
// to apps/bind-mock (later the real BIND) lives in provider/bind/ and the in-memory fake in
// provider/fake/. payers, payins, payouts and reconciliation depend on this package.
//
// Uncertain is not "no" (MO4): a timeout, network error or 5xx is ErrUncertain, never a
// failure; callers keep the hold and re-ask (the payouts sweep).
package psp
