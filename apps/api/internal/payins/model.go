package payins

import (
	"time"

	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
)

// Status of a payin (DB CHECK payins.status).
type Status string

// Payin statuses.
const (
	StatusReceived Status = "received"
	StatusCredited Status = "credited"
	StatusFailed   Status = "failed"
)

// Origin is who sent the money (R-IN-2).
type Origin struct {
	Name *string
	CUIT *string
	CBU  *string
}

// Payin is a credited (or failed) incoming transfer.
type Payin struct {
	ID                 uuid.UUID
	OrgID              uuid.UUID
	AccountID          uuid.UUID
	PayerID            *uuid.UUID
	CVUID              uuid.UUID
	Gross              money.Amount
	Fee                money.Amount
	Net                money.Amount
	Origin             Origin
	ProviderTransferID string
	Status             Status
	CreatedAt          time.Time
	CreditedAt         *time.Time
}

// WebhookDelivery is a signed PSP webhook exactly as received.
type WebhookDelivery struct {
	RawBody   []byte
	Signature string
	Timestamp string
}
