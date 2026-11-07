package orgs

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/identity"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/audit"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/config"
)

// Service is the orgs API used by the handler and by other features.
type Service interface {
	// List returns the caller's orgs (R-ORG-1).
	List(ctx context.Context, p auth.Principal) ([]Organization, error)
	// Create creates an org and makes the caller its admin.
	Create(ctx context.Context, p auth.Principal, in CreateInput) (Organization, error)
	// Get returns the principal's org (p.OrgID).
	Get(ctx context.Context, p auth.Principal) (Organization, error)
	// ListMembers lists the members of p.OrgID.
	ListMembers(ctx context.Context, p auth.Principal) ([]Member, error)
	// AddMember adds a registered user with a role not above the caller's (T4).
	AddMember(ctx context.Context, p auth.Principal, in MemberInput) (Member, error)
	// CreateInvite emails a single-use invite valid 7 days (R-ORG-2).
	CreateInvite(ctx context.Context, p auth.Principal, in MemberInput) (Invite, error)
	// AcceptInvite makes the caller a member (token hashed, single use, email must match).
	AcceptInvite(ctx context.Context, p auth.Principal, token string) (identity.MembershipSummary, error)

	// MembershipsForUser implements identity.Memberships.
	MembershipsForUser(ctx context.Context, userID uuid.UUID) ([]identity.MembershipSummary, error)
	// SetKYCStatus records the org's KYC status inside the kyc transaction (kyc.OrgStatus).
	SetKYCStatus(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, status string) error
	// OrgCUIT returns the org's CUIT for the ERP identity lookup ("" when it has none).
	OrgCUIT(ctx context.Context, orgID uuid.UUID) (string, error)
}

// Deps are the collaborators of the orgs service.
type Deps struct {
	Pool   *pgxpool.Pool
	Clock  clock.Clock
	Audit  *audit.Logger
	Users  Users
	Mailer InviteMailer
	Config config.Config
}

type service struct {
	deps Deps
}

// NewService returns the orgs service.
func NewService(deps Deps) Service { return &service{deps: deps} }

// notImplemented is the E0 stub answer; lane A deletes it with its last stub.
func (s *service) notImplemented() error {
	_ = s.deps
	return apperr.NotImplemented()
}

func (s *service) List(context.Context, auth.Principal) ([]Organization, error) {
	return nil, s.notImplemented()
}

func (s *service) Create(context.Context, auth.Principal, CreateInput) (Organization, error) {
	return Organization{}, s.notImplemented()
}

func (s *service) Get(context.Context, auth.Principal) (Organization, error) {
	return Organization{}, s.notImplemented()
}

func (s *service) ListMembers(context.Context, auth.Principal) ([]Member, error) {
	return nil, s.notImplemented()
}

func (s *service) AddMember(context.Context, auth.Principal, MemberInput) (Member, error) {
	return Member{}, s.notImplemented()
}

func (s *service) CreateInvite(context.Context, auth.Principal, MemberInput) (Invite, error) {
	return Invite{}, s.notImplemented()
}

func (s *service) AcceptInvite(context.Context, auth.Principal, string) (identity.MembershipSummary, error) {
	return identity.MembershipSummary{}, s.notImplemented()
}

func (s *service) MembershipsForUser(context.Context, uuid.UUID) ([]identity.MembershipSummary, error) {
	return nil, s.notImplemented()
}

func (s *service) SetKYCStatus(context.Context, pgx.Tx, uuid.UUID, string) error {
	return s.notImplemented()
}

func (s *service) OrgCUIT(context.Context, uuid.UUID) (string, error) {
	return "", s.notImplemented()
}
