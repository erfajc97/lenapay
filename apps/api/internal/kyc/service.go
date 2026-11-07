package kyc

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/audit"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
)

// Service is the KYC API used by the handler.
type Service interface {
	// Get returns the case of p.OrgID.
	Get(ctx context.Context, p auth.Principal) (Case, error)
	// Update replaces the case data while draft or rejected.
	Update(ctx context.Context, p auth.Principal, in UpdateInput) (Case, error)
	// UploadDocument stores a capped, type-sniffed file privately (A13).
	UploadDocument(ctx context.Context, p auth.Principal, in DocumentUpload) (Document, error)
	// Submit moves a complete case to submitted and runs the vendor check.
	Submit(ctx context.Context, p auth.Principal) (Case, error)
	// Decide records the ops decision (internal API key route); approval runs the hooks.
	Decide(ctx context.Context, caseID uuid.UUID, d Decision) (Case, error)
}

// Deps are the collaborators of the KYC service.
type Deps struct {
	Pool      *pgxpool.Pool
	Clock     clock.Clock
	Audit     *audit.Logger
	Provider  Provider
	Documents DocumentStore
	Orgs      OrgStatus
	OnApprove []ApprovalHook
}

type service struct {
	deps Deps
}

// NewService returns the KYC service.
func NewService(deps Deps) Service { return &service{deps: deps} }

// notImplemented is the E0 stub answer; lane A deletes it with its last stub.
func (s *service) notImplemented() error {
	_ = s.deps
	return apperr.NotImplemented()
}

func (s *service) Get(context.Context, auth.Principal) (Case, error) {
	return Case{}, s.notImplemented()
}

func (s *service) Update(context.Context, auth.Principal, UpdateInput) (Case, error) {
	return Case{}, s.notImplemented()
}

func (s *service) UploadDocument(context.Context, auth.Principal, DocumentUpload) (Document, error) {
	return Document{}, s.notImplemented()
}

func (s *service) Submit(context.Context, auth.Principal) (Case, error) {
	return Case{}, s.notImplemented()
}

func (s *service) Decide(context.Context, uuid.UUID, Decision) (Case, error) {
	return Case{}, s.notImplemented()
}
