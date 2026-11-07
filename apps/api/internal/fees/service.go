package fees

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
)

// Service is the fee calculator used by payins and payouts (no HTTP operations).
type Service interface {
	// Quote returns the fee of op on amount for orgID's plan in amount's currency.
	Quote(ctx context.Context, orgID uuid.UUID, op Operation, amount money.Amount) (Quote, error)
	// EnsureDefaultPlan creates the default plan if missing (cmd/seed and tests).
	EnsureDefaultPlan(ctx context.Context) error
}

// Deps are the collaborators of the fees service.
type Deps struct {
	Pool *pgxpool.Pool
}

type service struct {
	deps Deps
}

// NewService returns the fees service.
func NewService(deps Deps) Service { return &service{deps: deps} }

// notImplemented is the E0 stub answer; lane B deletes it with its last stub.
func (s *service) notImplemented() error {
	_ = s.deps
	return apperr.NotImplemented()
}

func (s *service) Quote(context.Context, uuid.UUID, Operation, money.Amount) (Quote, error) {
	return Quote{}, s.notImplemented()
}

func (s *service) EnsureDefaultPlan(context.Context) error { return s.notImplemented() }
