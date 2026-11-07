package erplink

import (
	"context"

	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/config"
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

// Orgs answers the org's CUIT (implemented by orgs.Service).
type Orgs interface {
	OrgCUIT(ctx context.Context, orgID uuid.UUID) (string, error)
}
