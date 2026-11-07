package fees

import (
	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
)

// Operation is what is charged.
type Operation string

// Charged operations.
const (
	OperationPayin  Operation = "payin"
	OperationPayout Operation = "payout"
)

// Quote is a computed fee with the plan values it came from (recorded at request time, MO5).
type Quote struct {
	Fee        money.Amount
	PlanID     uuid.UUID
	Bps        int64
	FixedMinor int64
}
