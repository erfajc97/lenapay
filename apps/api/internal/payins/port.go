package payins

import (
	"context"

	"github.com/erfajc97/lenapay/apps/api/internal/orgs"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
)

// Members lists who is notified of a payin: the org's members (orgs.Service).
type Members interface {
	ListMembers(ctx context.Context, p auth.Principal) ([]orgs.Member, error)
}
