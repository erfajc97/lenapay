package notifications

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/jobs"
)

// Service is the notifications API used by the handler and by other features.
type Service interface {
	// RegisterPushToken stores (or moves) an Expo push token to the caller.
	RegisterPushToken(ctx context.Context, p auth.Principal, token string, platform Platform) error
	// Notify writes the outbox inside the caller's transaction and queues delivery.
	Notify(ctx context.Context, tx pgx.Tx, n Notification) error
	// Deliver is the job that drains the outbox through the Sender.
	Deliver(ctx context.Context) error
	// SendEmailCode implements identity.CodeSender.
	SendEmailCode(ctx context.Context, email, code string) error
	// SendInvite implements orgs.InviteMailer.
	SendInvite(ctx context.Context, email, orgName, token string) error
}

// Deps are the collaborators of the notifications service.
type Deps struct {
	Pool   *pgxpool.Pool
	Clock  clock.Clock
	Jobs   jobs.Enqueuer
	Sender Sender
}

type service struct {
	deps Deps
}

// NewService returns the notifications service.
func NewService(deps Deps) Service { return &service{deps: deps} }

// notImplemented is the E0 stub answer; lane C deletes it with its last stub.
func (s *service) notImplemented() error {
	_ = s.deps
	return apperr.NotImplemented()
}

func (s *service) RegisterPushToken(context.Context, auth.Principal, string, Platform) error {
	return s.notImplemented()
}

func (s *service) Notify(context.Context, pgx.Tx, Notification) error { return s.notImplemented() }

func (s *service) Deliver(context.Context) error { return s.notImplemented() }

func (s *service) SendEmailCode(context.Context, string, string) error { return s.notImplemented() }

func (s *service) SendInvite(context.Context, string, string, string) error {
	return s.notImplemented()
}
