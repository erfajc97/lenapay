package movements

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/page"
)

// Service is the movements read API.
type Service interface {
	// List returns p.OrgID's movements, newest first.
	List(ctx context.Context, p auth.Principal, f Filter) (page.Page[Movement], error)
}

// Deps are the collaborators of the movements service.
type Deps struct {
	Pool *pgxpool.Pool
	// Location is the business time zone of Filter.From/To (A6).
	Location *time.Location
}

type service struct {
	deps Deps
}

// NewService returns the movements service.
func NewService(deps Deps) Service { return &service{deps: deps} }

// notImplemented is the E0 stub answer; lane B deletes it with its last stub.
func (s *service) notImplemented() error {
	_ = s.deps
	return apperr.NotImplemented()
}

func (s *service) List(context.Context, auth.Principal, Filter) (page.Page[Movement], error) {
	return page.Page[Movement]{}, s.notImplemented()
}
