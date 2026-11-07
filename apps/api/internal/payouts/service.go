package payouts

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/accounts"
	"github.com/erfajc97/lenapay/apps/api/internal/fees"
	"github.com/erfajc97/lenapay/apps/api/internal/ledger"
	"github.com/erfajc97/lenapay/apps/api/internal/notifications"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/audit"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/idempotency"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/jobs"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/page"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
)

// Service is the payouts API used by the handler, the worker and reconciliation.
type Service interface {
	ResolveDestination(ctx context.Context, p auth.Principal, to string) (psp.Destination, error)
	Quote(ctx context.Context, p auth.Principal, in QuoteInput) (Quote, error)
	// Create holds amount + fee and records the payout (idempotent by key, MO2/MO3/MO5).
	Create(ctx context.Context, p auth.Principal, in CreateInput) (Payout, error)
	Get(ctx context.Context, p auth.Principal, payoutID uuid.UUID) (Payout, error)
	// Approve is 4-eyes (payout.self_approval_forbidden for the creator) and queues dispatch.
	Approve(ctx context.Context, p auth.Principal, payoutID uuid.UUID, idempotencyKey string) (Payout, error)
	Reject(ctx context.Context, p auth.Principal, payoutID uuid.UUID, reason string) (Payout, error)
	// Cancel lets the creator cancel while pending approval (releases the hold).
	Cancel(ctx context.Context, p auth.Principal, payoutID uuid.UUID) (Payout, error)
	ListApprovals(ctx context.Context, p auth.Principal, req page.Request) (page.Page[Payout], error)
	// Proof renders the PDF proof of a settled payout (R-OUT-6).
	Proof(ctx context.Context, p auth.Principal, payoutID uuid.UUID) ([]byte, error)

	// Dispatch is the dispatch_payout job: one PSP attempt, then settle, fail or stay sending.
	Dispatch(ctx context.Context, payoutID uuid.UUID) error
	// Sweep is the periodic job that re-reads `sending` payouts at the PSP (R-OUT-5).
	Sweep(ctx context.Context) error
	// SettledBetween lists settled payouts of a currency in [from, to) (reconciliation).
	SettledBetween(ctx context.Context, currency money.Currency, from, to time.Time) ([]Payout, error)
}

// Deps are the collaborators of the payouts service.
type Deps struct {
	Pool          *pgxpool.Pool
	Clock         clock.Clock
	Audit         *audit.Logger
	Idempotency   *idempotency.Store
	Jobs          jobs.Enqueuer
	PSP           psp.Provider
	Accounts      accounts.Service
	Ledger        ledger.Service
	Fees          fees.Service
	Notifications notifications.Service
}

type service struct {
	deps Deps
}

// NewService returns the payouts service.
func NewService(deps Deps) Service { return &service{deps: deps} }

// notImplemented is the E0 stub answer; lane D deletes it with its last stub.
func (s *service) notImplemented() error {
	_ = s.deps
	return apperr.NotImplemented()
}

func (s *service) ResolveDestination(context.Context, auth.Principal, string) (psp.Destination, error) {
	return psp.Destination{}, s.notImplemented()
}

func (s *service) Quote(context.Context, auth.Principal, QuoteInput) (Quote, error) {
	return Quote{}, s.notImplemented()
}

func (s *service) Create(context.Context, auth.Principal, CreateInput) (Payout, error) {
	return Payout{}, s.notImplemented()
}

func (s *service) Get(context.Context, auth.Principal, uuid.UUID) (Payout, error) {
	return Payout{}, s.notImplemented()
}

func (s *service) Approve(context.Context, auth.Principal, uuid.UUID, string) (Payout, error) {
	return Payout{}, s.notImplemented()
}

func (s *service) Reject(context.Context, auth.Principal, uuid.UUID, string) (Payout, error) {
	return Payout{}, s.notImplemented()
}

func (s *service) Cancel(context.Context, auth.Principal, uuid.UUID) (Payout, error) {
	return Payout{}, s.notImplemented()
}

func (s *service) ListApprovals(context.Context, auth.Principal, page.Request) (page.Page[Payout], error) {
	return page.Page[Payout]{}, s.notImplemented()
}

func (s *service) Proof(context.Context, auth.Principal, uuid.UUID) ([]byte, error) {
	return nil, s.notImplemented()
}

func (s *service) Dispatch(context.Context, uuid.UUID) error { return s.notImplemented() }

func (s *service) Sweep(context.Context) error { return s.notImplemented() }

func (s *service) SettledBetween(context.Context, money.Currency, time.Time, time.Time) ([]Payout, error) {
	return nil, s.notImplemented()
}
