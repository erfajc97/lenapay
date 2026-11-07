package payins

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/fees"
	"github.com/erfajc97/lenapay/apps/api/internal/ledger"
	"github.com/erfajc97/lenapay/apps/api/internal/notifications"
	"github.com/erfajc97/lenapay/apps/api/internal/payers"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/audit"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/config"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/jobs"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
)

// Service is the payins API used by the handlers, the worker and reconciliation.
type Service interface {
	// ReceiveWebhook verifies, stores (deduplicated) and enqueues a PSP transfer event.
	ReceiveWebhook(ctx context.Context, d WebhookDelivery) error
	// ProcessTransfer is the job: re-read at the PSP, credit through the ledger, notify.
	ProcessTransfer(ctx context.Context, providerTransferID string) error
	// Get returns a payin of p.OrgID (payin.not_found when missing or foreign).
	Get(ctx context.Context, p auth.Principal, payinID uuid.UUID) (Payin, error)
	// CreditedBetween lists credited payins of a currency in [from, to) (reconciliation).
	CreditedBetween(ctx context.Context, currency money.Currency, from, to time.Time) ([]Payin, error)
}

// Deps are the collaborators of the payins service.
type Deps struct {
	Pool          *pgxpool.Pool
	Clock         clock.Clock
	Audit         *audit.Logger
	Jobs          jobs.Enqueuer
	PSP           psp.Provider
	Payers        payers.Service
	Ledger        ledger.Service
	Fees          fees.Service
	Notifications notifications.Service
	WebhookSecret config.Secret
	Tolerance     time.Duration
}

type service struct {
	deps Deps
}

// NewService returns the payins service.
func NewService(deps Deps) Service { return &service{deps: deps} }

// notImplemented is the E0 stub answer; lane C deletes it with its last stub.
func (s *service) notImplemented() error {
	_ = s.deps
	return apperr.NotImplemented()
}

func (s *service) ReceiveWebhook(context.Context, WebhookDelivery) error {
	return s.notImplemented()
}

func (s *service) ProcessTransfer(context.Context, string) error { return s.notImplemented() }

func (s *service) Get(context.Context, auth.Principal, uuid.UUID) (Payin, error) {
	return Payin{}, s.notImplemented()
}

func (s *service) CreditedBetween(context.Context, money.Currency, time.Time, time.Time) ([]Payin, error) {
	return nil, s.notImplemented()
}
