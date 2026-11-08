package kyc

import (
	"context"
	"io"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"lenapay/apps/api/internal/orgs"
)

// Provider is the identity-verification vendor (RENAPER validation later; provider/fake now:
// approves valid-looking data, rejects KYC_FAKE_REJECTED_DNIS).
type Provider interface {
	VerifyPerson(ctx context.Context, check PersonCheck) (Verdict, error)
	VerifyCompany(ctx context.Context, check CompanyCheck) (Verdict, error)
}

// PersonCheck is what the vendor validates for a person or a representative.
type PersonCheck struct {
	Reference string // our case id: the vendor dedupes by it
	DNI       string
	FullName  string
	SelfieKey string
}

// CompanyCheck is what the vendor validates for a company.
type CompanyCheck struct {
	Reference string
	CUIT      string
	LegalName string
}

// Verdict is the vendor's answer.
type Verdict struct {
	Approved    bool
	Reason      string
	ProviderRef string
}

// DocumentStore keeps KYC files in private storage (A13); keys are built by the service as
// org/<orgId>/kyc/<uuid>.
type DocumentStore interface {
	Put(ctx context.Context, key, contentType string, content io.Reader) error
}

// OrgStatus records the org's KYC status and, on approval, its verified identity (implemented
// by orgs.Service), both inside the decision transaction.
type OrgStatus interface {
	SetKYCStatus(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, status string) error
	ApplyVerifiedIdentity(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, id orgs.VerifiedIdentity) error
}

// ApprovalHook runs inside the approval transaction (accounts.Service, payers.Service).
// orgKind is company or personal.
type ApprovalHook interface {
	OnKYCApproved(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, orgKind string) error
}
