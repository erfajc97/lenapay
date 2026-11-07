package erplink

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/accounts"
	"github.com/erfajc97/lenapay/apps/api/internal/payers"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/audit"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/jobs"
)

// Service is the ERP link API used by the handler and the worker.
type Service interface {
	// Candidate looks the org's CUIT up in the identity index (R-ERP-1).
	Candidate(ctx context.Context, p auth.Principal) (Candidate, error)
	// Link checks the ERP credentials, stores the link and queues the import (R-ERP-2).
	Link(ctx context.Context, p auth.Principal, c Credentials) (Link, error)
	// Get returns the org's link (erp_link.not_found when none).
	Get(ctx context.Context, p auth.Principal) (Link, error)
	// Import is the erp_import job: projects → accounts, buyers → payers (R-ERP-3).
	Import(ctx context.Context, orgID uuid.UUID) error
	// LoadIdentityIndex refreshes erp_identity_index from the Directory (seed, schedule).
	LoadIdentityIndex(ctx context.Context) error
}

// Deps are the collaborators of the ERP link service.
type Deps struct {
	Pool      *pgxpool.Pool
	Clock     clock.Clock
	Audit     *audit.Logger
	Jobs      jobs.Enqueuer
	Directory Directory
	Orgs      Orgs
	Accounts  accounts.Service
	Payers    payers.Service
}

type service struct {
	deps Deps
}

// NewService returns the ERP link service.
func NewService(deps Deps) Service { return &service{deps: deps} }

// notImplemented is the E0 stub answer; lane D deletes it with its last stub.
func (s *service) notImplemented() error {
	_ = s.deps
	return apperr.NotImplemented()
}

func (s *service) Candidate(context.Context, auth.Principal) (Candidate, error) {
	return Candidate{}, s.notImplemented()
}

func (s *service) Link(context.Context, auth.Principal, Credentials) (Link, error) {
	return Link{}, s.notImplemented()
}

func (s *service) Get(context.Context, auth.Principal) (Link, error) {
	return Link{}, s.notImplemented()
}

func (s *service) Import(context.Context, uuid.UUID) error { return s.notImplemented() }

func (s *service) LoadIdentityIndex(context.Context) error { return s.notImplemented() }
