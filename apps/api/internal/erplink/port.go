package erplink

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/accounts"
	"github.com/erfajc97/lenapay/apps/api/internal/payers"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/config"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
)

// Directory is the ERP port (JSON fixtures in provider/fake now; a CDC read model or the ERP
// API later, architecture §8).
type Directory interface {
	// IdentityIndex returns every known identity (loaded into erp_identity_index).
	IdentityIndex(ctx context.Context) ([]IdentityEntry, error)
	// VerifyCredentials checks an ERP user of erpOrgID (erp_link.credentials_invalid otherwise).
	VerifyCredentials(ctx context.Context, erpOrgID, username string, password config.Secret) error
	// Projects lists the ERP org's projects.
	Projects(ctx context.Context, erpOrgID string) ([]Project, error)
	// Buyers lists the ERP org's buyers with their project.
	Buyers(ctx context.Context, erpOrgID string) ([]Buyer, error)
	// Installments lists one buyer's installments (cuotas) by number; none for an unknown buyer.
	Installments(ctx context.Context, erpOrgID, buyerRef string) ([]Installment, error)
}

// IdentityEntry is one row of the identity index.
type IdentityEntry struct {
	DocType     string // cuit | cuil | dni
	DocNumber   string
	ERPOrgID    string
	ERPClientID *string
	DisplayName string
}

// Project is an ERP project (becomes a project account).
type Project struct {
	Ref  string
	Name string
}

// Buyer is an ERP buyer (becomes a payer under its project's account).
type Buyer struct {
	Ref        string
	ProjectRef string
	Name       string
	DocType    string
	DocNumber  string
	Email      *string
}

// InstallmentStatus is the ERP's status of an installment.
type InstallmentStatus string

// Installment statuses (contract InstallmentStatus).
const (
	InstallmentPending InstallmentStatus = "pending"
	InstallmentPaid    InstallmentStatus = "paid"
	InstallmentOverdue InstallmentStatus = "overdue"
)

// Installment is one cuota of a buyer's payment plan, as the ERP holds it.
type Installment struct {
	Number  int
	DueDate time.Time // a calendar date (UTC midnight)
	Amount  money.Amount
	Status  InstallmentStatus
	// AdjustmentIndex is the index the amount follows (CAC or UVA); nil for a fixed amount.
	AdjustmentIndex *string
}

// Orgs answers the org's CUIT (implemented by orgs.Service).
type Orgs interface {
	OrgCUIT(ctx context.Context, orgID uuid.UUID) (string, error)
}

// ProjectImporter creates the project account of an ERP project (implemented by accounts.Service;
// idempotent by the ERP project ref).
type ProjectImporter interface {
	ImportProject(ctx context.Context, orgID uuid.UUID, name, erpProjectRef string) (accounts.Account, error)
}

// PayerImporter creates the payer of an ERP buyer (implemented by payers.Service; idempotent by
// the ERP buyer ref).
type PayerImporter interface {
	ImportPayer(ctx context.Context, orgID uuid.UUID, in payers.ImportInput) (payers.Payer, error)
}
