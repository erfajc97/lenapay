package accounts

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/ledger"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/audit"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/idempotency"
)

// Service is the accounts API used by the handler and by other features.
type Service interface {
	// List returns p.OrgID's accounts with balances.
	List(ctx context.Context, p auth.Principal, includeArchived bool) ([]Account, error)
	// Create opens a project account (R-ACC-3).
	Create(ctx context.Context, p auth.Principal, in CreateInput) (Account, error)
	// Update renames or archives an account (archive only with zero balance).
	Update(ctx context.Context, p auth.Principal, accountID uuid.UUID, in UpdateInput) (Account, error)
	// Summary returns totals per currency and per account.
	Summary(ctx context.Context, p auth.Principal) (Summary, error)
	// Transfer posts an internal transfer (R-ACC-4, idempotent, MO2/MO3).
	Transfer(ctx context.Context, p auth.Principal, in TransferInput) (Transfer, error)

	// Get returns an account of orgID (account.not_found when missing or foreign, T2).
	Get(ctx context.Context, orgID, accountID uuid.UUID) (Account, error)
	// OnKYCApproved opens the main (company) or personal account (kyc.ApprovalHook).
	OnKYCApproved(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, orgKind string) error
	// ImportProject creates (or returns) the project account of an ERP project (erplink import).
	ImportProject(ctx context.Context, orgID uuid.UUID, name, erpProjectRef string) (Account, error)
}

// Deps are the collaborators of the accounts service.
type Deps struct {
	Pool        *pgxpool.Pool
	Clock       clock.Clock
	Audit       *audit.Logger
	Idempotency *idempotency.Store
	Ledger      ledger.Service
}

type service struct {
	deps Deps
}

// NewService returns the accounts service.
func NewService(deps Deps) Service { return &service{deps: deps} }

// notImplemented is the E0 stub answer; lane B deletes it with its last stub.
func (s *service) notImplemented() error {
	_ = s.deps
	return apperr.NotImplemented()
}

func (s *service) List(context.Context, auth.Principal, bool) ([]Account, error) {
	return nil, s.notImplemented()
}

func (s *service) Create(context.Context, auth.Principal, CreateInput) (Account, error) {
	return Account{}, s.notImplemented()
}

func (s *service) Update(context.Context, auth.Principal, uuid.UUID, UpdateInput) (Account, error) {
	return Account{}, s.notImplemented()
}

func (s *service) Summary(context.Context, auth.Principal) (Summary, error) {
	return Summary{}, s.notImplemented()
}

func (s *service) Transfer(context.Context, auth.Principal, TransferInput) (Transfer, error) {
	return Transfer{}, s.notImplemented()
}

func (s *service) Get(context.Context, uuid.UUID, uuid.UUID) (Account, error) {
	return Account{}, s.notImplemented()
}

func (s *service) OnKYCApproved(context.Context, pgx.Tx, uuid.UUID, string) error {
	return s.notImplemented()
}

func (s *service) ImportProject(context.Context, uuid.UUID, string, string) (Account, error) {
	return Account{}, s.notImplemented()
}
