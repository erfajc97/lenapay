package payouts

import (
	"time"

	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
)

// Status of a payout (DB CHECK payouts.status).
type Status string

// Payout statuses: pending_approval → approved → sending → settled | failed; rejected and
// cancelled end a pending payout.
const (
	StatusPendingApproval Status = "pending_approval"
	StatusApproved        Status = "approved"
	StatusSending         Status = "sending"
	StatusSettled         Status = "settled"
	StatusFailed          Status = "failed"
	StatusRejected        Status = "rejected"
	StatusCancelled       Status = "cancelled"
)

// QuoteInput asks for the fee and total of a payout before confirming (R-OUT-2).
type QuoteInput struct {
	AccountID uuid.UUID
	Amount    money.Amount
	To        string
}

// Quote is the fee, total and resolved destination.
type Quote struct {
	Amount         money.Amount
	Fee            money.Amount
	Total          money.Amount
	Destination    psp.Destination
	AvailableAfter money.Amount
}

// CreateInput creates a payout (R-OUT-1/3).
type CreateInput struct {
	AccountID      uuid.UUID
	Amount         money.Amount
	To             string
	Concept        string
	IdempotencyKey string // required (MO2)
}

// Event is one status change.
type Event struct {
	FromStatus *Status
	ToStatus   Status
	ActorID    *uuid.UUID
	Reason     *string
	CreatedAt  time.Time
}

// Payout is a payout with its history.
type Payout struct {
	ID           uuid.UUID
	OrgID        uuid.UUID
	AccountID    uuid.UUID
	Amount       money.Amount
	Fee          money.Amount
	Destination  psp.Destination
	Concept      string
	Status       Status
	CreatedBy    uuid.UUID
	ApprovedBy   *uuid.UUID
	RejectReason *string
	FailureCode  *string
	Events       []Event
	CreatedAt    time.Time
	UpdatedAt    time.Time
	SettledAt    *time.Time
}
