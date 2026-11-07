package psp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
)

// Errors every adapter maps its failures to.
var (
	// ErrUncertain: timeout, network error or 5xx. The effect may or may not have happened.
	ErrUncertain = errors.New("psp: outcome uncertain")
	// ErrNotFound: the provider does not know the reference (transfer, CVU, destination).
	ErrNotFound = errors.New("psp: not found")
	// ErrRejected: the provider refused the request (invalid alias, closed account, ...).
	ErrRejected = errors.New("psp: rejected")
	// ErrAliasTaken: the alias belongs to another account. It is also ErrRejected.
	ErrAliasTaken = fmt.Errorf("%w: alias taken", ErrRejected)
)

// Provider is the PSP port. Every call takes ctx (its deadline bounds the HTTP call) and our
// own reference where the provider supports idempotency.
type Provider interface {
	// CreateCVU issues a CVU (and alias) for a payer or a personal account (R-CVU-1, R-CVU-3).
	CreateCVU(ctx context.Context, req CreateCVURequest) (CVUAccount, error)
	// UpdateAlias changes the alias of a CVU (R-CVU-2).
	UpdateAlias(ctx context.Context, providerRef, alias string) error
	// DisableCVU stops a CVU from receiving transfers (R-CVU-2).
	DisableCVU(ctx context.Context, providerRef string) error
	// GetTransfer re-reads an incoming transfer announced by a webhook (R-IN-1, MO4).
	GetTransfer(ctx context.Context, transferID string) (IncomingTransfer, error)
	// ResolveDestination looks up a CBU, CVU or alias and its holder (R-OUT-1).
	ResolveDestination(ctx context.Context, to string) (Destination, error)
	// SendTransfer starts an outgoing transfer; Reference is ONE per attempt (MO5).
	SendTransfer(ctx context.Context, req SendTransferRequest) (TransferResult, error)
	// TransferByReference re-reads an outgoing transfer by our reference (the sweep, R-OUT-5).
	TransferByReference(ctx context.Context, reference string) (TransferResult, error)
	// Statement lists the master account movements of one day (R-REC-1).
	Statement(ctx context.Context, currency money.Currency, day time.Time) ([]StatementLine, error)
}

// CreateCVURequest asks for a new CVU.
type CreateCVURequest struct {
	Reference  string // our CVU id: the provider dedupes by it
	Currency   money.Currency
	HolderName string
	HolderDoc  string // CUIT/CUIL/DNI digits
	Alias      *string
}

// CVUAccount is an issued CVU.
type CVUAccount struct {
	CVU         string
	Alias       string
	ProviderRef string
}

// IncomingTransfer is a transfer received on one of our CVUs, as the provider reports it.
type IncomingTransfer struct {
	ID         string
	CVU        string
	Amount     money.Amount
	OriginName string
	OriginCUIT string
	OriginCBU  string
	OccurredAt time.Time
}

// DestinationType is what the user typed.
type DestinationType string

// Destination types.
const (
	DestinationCBU   DestinationType = "cbu"
	DestinationCVU   DestinationType = "cvu"
	DestinationAlias DestinationType = "alias"
)

// Destination is a resolved payout destination.
type Destination struct {
	Type          DestinationType
	Value         string
	AccountNumber string
	HolderName    string
	HolderCUIT    string
	BankName      string
}

// SendTransferRequest is an outgoing transfer attempt.
type SendTransferRequest struct {
	Reference     string // ONE per attempt (payouts.provider_attempt_id)
	AccountNumber string
	Amount        money.Amount
	Concept       string
}

// TransferStatus is the provider status of an outgoing transfer.
type TransferStatus string

// Outgoing transfer statuses.
const (
	TransferPending  TransferStatus = "pending"
	TransferSettled  TransferStatus = "settled"
	TransferRejected TransferStatus = "rejected"
)

// TransferResult is the provider's view of an outgoing transfer.
type TransferResult struct {
	ProviderID string
	Status     TransferStatus
	Reason     string
}

// Direction of a statement line.
type Direction string

// Statement directions.
const (
	DirectionIn  Direction = "in"
	DirectionOut Direction = "out"
)

// StatementLine is one movement of the master account.
type StatementLine struct {
	TransferID string
	Direction  Direction
	Amount     money.Amount
	OccurredAt time.Time
}
