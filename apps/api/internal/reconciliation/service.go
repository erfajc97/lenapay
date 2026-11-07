package reconciliation

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/payins"
	"github.com/erfajc97/lenapay/apps/api/internal/payouts"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/jobs"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/page"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
)

// Service is the reconciliation API used by the handler and the worker.
type Service interface {
	// ListRuns lists runs visible to p (newest first).
	ListRuns(ctx context.Context, p auth.Principal, currency *money.Currency, req page.Request) (page.Page[Run], error)
	// RequestRun queues a run (internal endpoint and the daily periodic job).
	RequestRun(ctx context.Context, in RunInput) (Run, error)
	// Reconcile is the reconcile_master job.
	Reconcile(ctx context.Context, runID uuid.UUID) error
}

// Deps are the collaborators of the reconciliation service.
type Deps struct {
	Pool    *pgxpool.Pool
	Clock   clock.Clock
	Jobs    jobs.Enqueuer
	PSP     psp.Provider
	Payins  payins.Service
	Payouts payouts.Service
}

type service struct {
	deps Deps
}

// NewService returns the reconciliation service.
func NewService(deps Deps) Service { return &service{deps: deps} }

// notImplemented is the E0 stub answer; lane D deletes it with its last stub.
func (s *service) notImplemented() error {
	_ = s.deps
	return apperr.NotImplemented()
}

func (s *service) ListRuns(context.Context, auth.Principal, *money.Currency, page.Request) (page.Page[Run], error) {
	return page.Page[Run]{}, s.notImplemented()
}

func (s *service) RequestRun(context.Context, RunInput) (Run, error) {
	return Run{}, s.notImplemented()
}

func (s *service) Reconcile(context.Context, uuid.UUID) error { return s.notImplemented() }
