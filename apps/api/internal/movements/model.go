package movements

import (
	"time"

	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/page"
)

// Type of movement (contract MovementType).
type Type string

// Movement types.
const (
	TypePayin            Type = "payin"
	TypePayout           Type = "payout"
	TypeInternalTransfer Type = "internal_transfer"
)

// Status of a movement (contract MovementStatus).
type Status string

// Movement statuses.
const (
	StatusPending   Status = "pending"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Direction of a movement for its account.
type Direction string

// Directions.
const (
	DirectionIn  Direction = "in"
	DirectionOut Direction = "out"
)

// Counterparty is the other side of a movement.
type Counterparty struct {
	Name, CUIT, CBU *string
}

// Movement is one line of the movements list.
type Movement struct {
	ID           uuid.UUID
	Type         Type
	Status       Status
	Direction    Direction
	AccountID    uuid.UUID
	Amount       money.Amount
	Fee          *money.Amount
	Counterparty Counterparty
	Concept      *string
	RefID        uuid.UUID
	CreatedAt    time.Time
}

// Filter narrows the list; From/To are business dates (inclusive) in the business time zone.
type Filter struct {
	AccountID *uuid.UUID
	Type      *Type
	Status    *Status
	Currency  *money.Currency
	From      *time.Time
	To        *time.Time
	Page      page.Request
}
