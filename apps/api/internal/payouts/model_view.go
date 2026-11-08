package payouts

import (
	"time"

	"github.com/google/uuid"

	"lenapay/apps/api/internal/platform/money"
	"lenapay/apps/api/internal/psp"
)

// PayoutView is a payout as the outside world sees it: the field allowlist of the contract's
// Payout (A5), with the same JSON names and field order, but owned by this feature so the
// realtime payload does not depend on the transport layer (AG1). Handlers copy it into the
// generated api.Payout; the realtime event marshals it as is. Field order is the JSON key
// order: keep it alphabetical like the generated type, the mobile app parses these bytes.
type PayoutView struct {
	AccountID    uuid.UUID         `json:"accountId"`
	Amount       MoneyView         `json:"amount"`
	ApprovedBy   *uuid.UUID        `json:"approvedBy"`
	Attachments  []AttachmentView  `json:"attachments"`
	Concept      string            `json:"concept"`
	CreatedAt    time.Time         `json:"createdAt"`
	CreatedBy    uuid.UUID         `json:"createdBy"`
	Destination  DestinationView   `json:"destination"`
	Events       []PayoutEventView `json:"events"`
	FailureCode  *string           `json:"failureCode"`
	Fee          MoneyView         `json:"fee"`
	ID           uuid.UUID         `json:"id"`
	Rail         Rail              `json:"rail"`
	RejectReason *string           `json:"rejectReason"`
	SettledAt    *time.Time        `json:"settledAt"`
	Status       Status            `json:"status"`
	Total        MoneyView         `json:"total"`
	UpdatedAt    time.Time         `json:"updatedAt"`
}

// MoneyView is the contract's Money: minor units and currency, never a float (P4).
type MoneyView struct {
	Currency money.Currency `json:"currency"`
	Minor    int64          `json:"minor"`
}

// DestinationView is the contract's Destination; BankName is null when the PSP gave none.
type DestinationView struct {
	AccountNumber string              `json:"accountNumber"`
	BankName      *string             `json:"bankName"`
	HolderCuit    string              `json:"holderCuit"`
	HolderName    string              `json:"holderName"`
	Type          psp.DestinationType `json:"type"`
	Value         string              `json:"value"`
}

// PayoutEventView is the contract's PayoutEvent (one status transition).
type PayoutEventView struct {
	ActorID    *uuid.UUID `json:"actorId"`
	CreatedAt  time.Time  `json:"createdAt"`
	FromStatus *Status    `json:"fromStatus"`
	Reason     *string    `json:"reason"`
	ToStatus   Status     `json:"toStatus"`
}

// AttachmentView is the contract's PayoutAttachment (the storage key never leaves the API).
type AttachmentView struct {
	ContentType string    `json:"contentType"`
	CreatedAt   time.Time `json:"createdAt"`
	FileName    string    `json:"fileName"`
	ID          uuid.UUID `json:"id"`
	SizeBytes   int64     `json:"sizeBytes"`
	UploadedBy  uuid.UUID `json:"uploadedBy"`
}

// NewPayoutView builds the allowlist of a payout field by field; every door that shows a
// payout (HTTP responses of payouts and payerview, the realtime event) starts here (G12).
// The lists are never null: a payout without history shows [].
func NewPayoutView(po Payout) PayoutView {
	total, err := po.Total()
	if err != nil { // amount + fee was summed when the hold was taken; it cannot overflow now
		total = po.Amount
	}
	events := make([]PayoutEventView, 0, len(po.Events))
	for _, e := range po.Events {
		events = append(events, PayoutEventView{
			ActorID: e.ActorID, CreatedAt: e.CreatedAt, FromStatus: e.FromStatus, Reason: e.Reason, ToStatus: e.ToStatus,
		})
	}
	attachments := make([]AttachmentView, 0, len(po.Attachments))
	for _, a := range po.Attachments {
		attachments = append(attachments, NewAttachmentView(a))
	}
	return PayoutView{
		Attachments: attachments,
		ID:          po.ID, AccountID: po.AccountID, Amount: moneyView(po.Amount), Fee: moneyView(po.Fee), Total: moneyView(total),
		Destination: destinationView(po.Destination), Concept: po.Concept, Status: po.Status, Rail: po.Rail,
		CreatedBy: po.CreatedBy, ApprovedBy: po.ApprovedBy, RejectReason: po.RejectReason, FailureCode: po.FailureCode,
		Events: events, CreatedAt: po.CreatedAt, UpdatedAt: po.UpdatedAt, SettledAt: po.SettledAt,
	}
}

// NewAttachmentView builds the allowlist of a stored attachment.
func NewAttachmentView(a Attachment) AttachmentView {
	return AttachmentView{
		ID: a.ID, FileName: a.FileName, ContentType: a.ContentType,
		SizeBytes: a.SizeBytes, UploadedBy: a.UploadedBy, CreatedAt: a.CreatedAt,
	}
}

func moneyView(a money.Amount) MoneyView {
	return MoneyView{Minor: a.Minor, Currency: a.Currency}
}

func destinationView(d psp.Destination) DestinationView {
	out := DestinationView{
		Type: d.Type, Value: d.Value, AccountNumber: d.AccountNumber,
		HolderName: d.HolderName, HolderCuit: d.HolderCUIT,
	}
	if d.BankName != "" {
		bank := d.BankName
		out.BankName = &bank
	}
	return out
}
