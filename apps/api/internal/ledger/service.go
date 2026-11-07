package ledger

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
)

// Service is the ledger API for other features (no HTTP operations of its own).
type Service interface {
	// EnsureSystemAccounts creates the system accounts and their ledger accounts in every
	// currency (idempotent; cmd/api and cmd/seed call it at boot).
	EnsureSystemAccounts(ctx context.Context) error
	// SystemAccountID returns the account id of a system account.
	SystemAccountID(ctx context.Context, code SystemCode) (uuid.UUID, error)
	// OpenAccount creates the ledger accounts (ARS and USD) of an org account inside tx.
	OpenAccount(ctx context.Context, tx pgx.Tx, accountID uuid.UUID) error
	// Post writes a balanced entry inside tx and updates the cached balances under lock.
	// It fails with ledger.insufficient_funds when a non-system leg would overdraw.
	Post(ctx context.Context, tx pgx.Tx, e Entry) (uuid.UUID, error)
	// Hold reserves funds inside tx (ledger.insufficient_funds when available is short).
	Hold(ctx context.Context, tx pgx.Tx, in HoldInput) (uuid.UUID, error)
	// ReleaseHold frees a hold only when it is known the effect did not happen (MO5).
	ReleaseHold(ctx context.Context, tx pgx.Tx, holdID uuid.UUID) error
	// CaptureHold consumes a hold and posts its entry in the same transaction.
	CaptureHold(ctx context.Context, tx pgx.Tx, holdID uuid.UUID, e Entry) (uuid.UUID, error)
	// Balances returns posted, held and available per account and currency.
	Balances(ctx context.Context, accountIDs []uuid.UUID) ([]Balance, error)
}

// Deps are the collaborators of the ledger service.
type Deps struct {
	Pool  *pgxpool.Pool
	Clock clock.Clock
}

type service struct {
	deps Deps
}

// NewService returns the ledger service.
func NewService(deps Deps) Service { return &service{deps: deps} }

// notImplemented is the E0 stub answer; lane B deletes it with its last stub.
func (s *service) notImplemented() error {
	_ = s.deps
	return apperr.NotImplemented()
}

func (s *service) EnsureSystemAccounts(context.Context) error { return s.notImplemented() }

func (s *service) SystemAccountID(context.Context, SystemCode) (uuid.UUID, error) {
	return uuid.Nil, s.notImplemented()
}

func (s *service) OpenAccount(context.Context, pgx.Tx, uuid.UUID) error { return s.notImplemented() }

func (s *service) Post(context.Context, pgx.Tx, Entry) (uuid.UUID, error) {
	return uuid.Nil, s.notImplemented()
}

func (s *service) Hold(context.Context, pgx.Tx, HoldInput) (uuid.UUID, error) {
	return uuid.Nil, s.notImplemented()
}

func (s *service) ReleaseHold(context.Context, pgx.Tx, uuid.UUID) error { return s.notImplemented() }

func (s *service) CaptureHold(context.Context, pgx.Tx, uuid.UUID, Entry) (uuid.UUID, error) {
	return uuid.Nil, s.notImplemented()
}

func (s *service) Balances(context.Context, []uuid.UUID) ([]Balance, error) {
	return nil, s.notImplemented()
}
