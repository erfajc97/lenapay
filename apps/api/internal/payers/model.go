package payers

import (
	"time"

	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/page"
)

// DocType of a payer's document.
type DocType string

// Document types (DB CHECK payers.doc_type).
const (
	DocCUIT DocType = "cuit"
	DocCUIL DocType = "cuil"
	DocDNI  DocType = "dni"
)

// CVU is a collection account number of a payer (or of a personal account when PayerID is nil).
type CVU struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	PayerID   *uuid.UUID
	AccountID uuid.UUID
	Currency  money.Currency
	Number    string
	Alias     *string
	Active    bool
	ShareLink string
	QRPayload string
	CreatedAt time.Time
}

// Payer is someone who pays an org (a buyer of a project).
type Payer struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	AccountID uuid.UUID
	Name      string
	DocType   DocType
	DocNumber string
	Email     *string
	ERPRef    *string
	CVUs      []CVU
	CreatedAt time.Time
}

// ListInput filters and pages payers.
type ListInput struct {
	AccountID *uuid.UUID
	Query     *string
	Page      page.Request
}

// CreateInput creates a payer under an account of the org.
type CreateInput struct {
	AccountID uuid.UUID
	Name      string
	DocType   DocType
	DocNumber string
	Email     *string
}

// UpdateInput patches a payer: nil keeps, ClearEmail sets email to null (A4).
type UpdateInput struct {
	Name       *string
	Email      *string
	ClearEmail bool
}

// CreateCVUInput issues a CVU for a payer.
type CreateCVUInput struct {
	Currency       money.Currency
	Alias          *string
	IdempotencyKey string // required (MO2): the PSP call is retried with the same reference
}

// ImportInput creates (or returns) a payer from an ERP buyer snapshot.
type ImportInput struct {
	AccountID uuid.UUID
	Name      string
	DocType   DocType
	DocNumber string
	Email     *string
	ERPRef    string
}
