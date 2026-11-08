package payouts

import (
	"io"
	"time"

	"github.com/google/uuid"

	"lenapay/apps/api/internal/platform/money"
	"lenapay/apps/api/internal/psp"
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

// Rail says how the money moves (DB CHECK payouts.rail).
type Rail string

// Rails: through the PSP, or on-us between two Lenapay accounts (never reaches the bank).
const (
	RailBank     Rail = "bank"
	RailInternal Rail = "internal"
)

// Field caps shared with the contract and the DB CHECKs of payouts (A3, G12).
const (
	maxConcept      = 140
	maxRejectReason = 280
	minIdemKey      = 16
	maxIdemKey      = 128
	maxHolderName   = 200
	maxBankName     = 120
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
	Rail           Rail
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

// AttachmentUpload is one uploaded supporting document; the service sniffs the real type.
type AttachmentUpload struct {
	FileName string
	Content  io.Reader
}

// Attachment is a stored supporting document (the storage key never leaves the API).
type Attachment struct {
	ID          uuid.UUID
	FileName    string
	ContentType string
	SizeBytes   int64
	UploadedBy  uuid.UUID
	CreatedAt   time.Time
}

// Payout is a payout with its history (Events and Attachments are filled by Get; lists leave
// them empty).
type Payout struct {
	ID           uuid.UUID
	OrgID        uuid.UUID
	AccountID    uuid.UUID
	Amount       money.Amount
	Fee          money.Amount
	Destination  psp.Destination
	Concept      string
	Status       Status
	Rail         Rail
	CreatedBy    uuid.UUID
	ApprovedBy   *uuid.UUID
	RejectReason *string
	FailureCode  *string
	Events       []Event
	Attachments  []Attachment
	CreatedAt    time.Time
	UpdatedAt    time.Time
	SettledAt    *time.Time
	// ProviderAttemptID is our reference of the last PSP attempt; ProviderTransferID is the
	// PSP's id once settled (reconciliation matches statement lines on either).
	ProviderAttemptID  *string
	ProviderTransferID *string
}

// Total is amount + fee: what is held on creation and debited on settlement.
func (p Payout) Total() (money.Amount, error) { return p.Amount.Add(p.Fee) }
