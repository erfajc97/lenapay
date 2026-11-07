package payers

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/accounts"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/audit"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/idempotency"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/jobs"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/page"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
)

// Service is the payers/CVU API used by the handler and by other features.
type Service interface {
	List(ctx context.Context, p auth.Principal, in ListInput) (page.Page[Payer], error)
	Create(ctx context.Context, p auth.Principal, in CreateInput) (Payer, error)
	Get(ctx context.Context, p auth.Principal, payerID uuid.UUID) (Payer, error)
	Update(ctx context.Context, p auth.Principal, payerID uuid.UUID, in UpdateInput) (Payer, error)
	// CreateCVU issues the payer's CVU in a currency at the PSP (one per payer per currency).
	CreateCVU(ctx context.Context, p auth.Principal, payerID uuid.UUID, in CreateCVUInput) (CVU, error)
	UpdateCVUAlias(ctx context.Context, p auth.Principal, cvuID uuid.UUID, alias string) (CVU, error)
	DisableCVU(ctx context.Context, p auth.Principal, cvuID uuid.UUID) error

	// CVUByNumber resolves a receiving CVU for payins (system scope; cvu.not_found otherwise).
	CVUByNumber(ctx context.Context, number string) (CVU, error)
	// ImportPayer creates or returns the payer of an ERP buyer (erplink import, idempotent by ERPRef).
	ImportPayer(ctx context.Context, orgID uuid.UUID, in ImportInput) (Payer, error)
	// OnKYCApproved queues the personal CVUs of a personal org (kyc.ApprovalHook).
	OnKYCApproved(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, orgKind string) error
}

// Deps are the collaborators of the payers service.
type Deps struct {
	Pool        *pgxpool.Pool
	Clock       clock.Clock
	Audit       *audit.Logger
	Idempotency *idempotency.Store
	Jobs        jobs.Enqueuer
	PSP         psp.Provider
	Accounts    accounts.Service
}

type service struct {
	deps Deps
}

// NewService returns the payers service.
func NewService(deps Deps) Service { return &service{deps: deps} }

// notImplemented is the E0 stub answer; lane C deletes it with its last stub.
func (s *service) notImplemented() error {
	_ = s.deps
	return apperr.NotImplemented()
}

func (s *service) List(context.Context, auth.Principal, ListInput) (page.Page[Payer], error) {
	return page.Page[Payer]{}, s.notImplemented()
}

func (s *service) Create(context.Context, auth.Principal, CreateInput) (Payer, error) {
	return Payer{}, s.notImplemented()
}

func (s *service) Get(context.Context, auth.Principal, uuid.UUID) (Payer, error) {
	return Payer{}, s.notImplemented()
}

func (s *service) Update(context.Context, auth.Principal, uuid.UUID, UpdateInput) (Payer, error) {
	return Payer{}, s.notImplemented()
}

func (s *service) CreateCVU(context.Context, auth.Principal, uuid.UUID, CreateCVUInput) (CVU, error) {
	return CVU{}, s.notImplemented()
}

func (s *service) UpdateCVUAlias(context.Context, auth.Principal, uuid.UUID, string) (CVU, error) {
	return CVU{}, s.notImplemented()
}

func (s *service) DisableCVU(context.Context, auth.Principal, uuid.UUID) error {
	return s.notImplemented()
}

func (s *service) CVUByNumber(context.Context, string) (CVU, error) {
	return CVU{}, s.notImplemented()
}

func (s *service) ImportPayer(context.Context, uuid.UUID, ImportInput) (Payer, error) {
	return Payer{}, s.notImplemented()
}

func (s *service) OnKYCApproved(context.Context, pgx.Tx, uuid.UUID, string) error {
	return s.notImplemented()
}
