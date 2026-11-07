package identity

import (
	"context"

	"github.com/google/uuid"
)

// Memberships lists the orgs of a user (implemented by orgs.Service).
type Memberships interface {
	MembershipsForUser(ctx context.Context, userID uuid.UUID) ([]MembershipSummary, error)
}

// MembershipsFunc adapts a function to Memberships. cmd/api uses it to break the
// identity ↔ orgs construction cycle (orgs needs identity.Service first).
type MembershipsFunc func(ctx context.Context, userID uuid.UUID) ([]MembershipSummary, error)

// MembershipsForUser implements Memberships.
func (f MembershipsFunc) MembershipsForUser(ctx context.Context, userID uuid.UUID) ([]MembershipSummary, error) {
	return f(ctx, userID)
}

// CodeSender emails verification codes (implemented by notifications.Service; the fake
// sender logs the code when DEV_INBOX_ENABLED, R-ID-1).
type CodeSender interface {
	SendEmailCode(ctx context.Context, email, code string) error
}
