package fees

import (
	"github.com/google/uuid"

	"lenapay/apps/api/internal/platform/money"
)

// Operation is what is charged.
type Operation string

// Charged operations.
const (
	OperationPayin  Operation = "payin"
	OperationPayout Operation = "payout"
	// OperationPayoutInternal is an on-us payout (to a Lenapay CVU, never through the bank):
	// fixed + bps of the plan's internal rates, 0 by default.
	OperationPayoutInternal Operation = "payout_internal"
)

// Rounding is the ONE rounding of every percentage fee (MO1): half a minor unit or more
// rounds up. Quote, hold and capture all use the stored quote, never a recomputation (MO5).
const Rounding = money.RoundHalfUp

// Rates are one currency of a fee plan.
type Rates struct {
	PayinBps           int64
	PayoutFixedMinor   int64
	PayoutBps          int64
	InternalFixedMinor int64
	InternalBps        int64
}

// DefaultRates seed the default plan per currency when it is missing. (proposal, G11 — the PRD
// asks for defaults per currency without values; the owner confirms or changes them in
// docs/progress.md.) ARS: payin 0.50 %, payout ARS 150.00 + 0.20 %. USD: payin 0.50 %, payout
// USD 1.00 + 0 %.
func DefaultRates() map[money.Currency]Rates {
	return map[money.Currency]Rates{
		money.ARS: {PayinBps: 50, PayoutFixedMinor: 15_000, PayoutBps: 20},
		money.USD: {PayinBps: 50, PayoutFixedMinor: 100, PayoutBps: 0},
	}
}

// Quote is a computed fee with the plan values it came from (recorded at request time, MO5).
type Quote struct {
	Fee        money.Amount
	PlanID     uuid.UUID
	Bps        int64
	FixedMinor int64
}
