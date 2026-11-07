package payers

import (
	"context"

	"github.com/erfajc97/lenapay/apps/api/internal/orgs"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
)

// Orgs is the part of orgs.Service payers needs: the org is the holder of its CVUs at the PSP
// (payers pay INTO the org's money, so the PSP shows the org's legal name and CUIT).
type Orgs interface {
	// Get returns p.OrgID's org as seen by member p.UserID.
	Get(ctx context.Context, p auth.Principal) (orgs.Organization, error)
	// ListMembers lists p.OrgID's members (the personal-CVU job has no user of its own).
	ListMembers(ctx context.Context, p auth.Principal) ([]orgs.Member, error)
}
