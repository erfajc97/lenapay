package orgs

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Users finds registered users (implemented by identity.Service).
type Users interface {
	UserIDByEmail(ctx context.Context, email string) (uuid.UUID, error)
}

// InviteMailer emails invitation links (implemented by notifications.Service).
type InviteMailer interface {
	SendInvite(ctx context.Context, email, orgName, token string) error
}

// KYCCases opens the onboarding case of a new org inside the creation transaction
// (implemented by kyc.Service).
type KYCCases interface {
	OpenCase(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, orgKind string) error
}

// KYCCasesFunc adapts a function to KYCCases. cmd/api uses it to break the orgs ↔ kyc
// construction cycle (kyc needs orgs.Service first).
type KYCCasesFunc func(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, orgKind string) error

// OpenCase implements KYCCases.
func (f KYCCasesFunc) OpenCase(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, orgKind string) error {
	return f(ctx, tx, orgID, orgKind)
}

// Names fills a user's full name from a verified identity (implemented by identity.Service):
// the person KYC approval names the owner of the personal org when they have no name yet.
type Names interface {
	FillFullNameIfEmpty(ctx context.Context, tx pgx.Tx, userID uuid.UUID, name string) error
}
