package identity

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

// SignupHook runs inside the signup transaction when a NEW user is inserted (never on a
// re-sent code): cmd/api wires the personal org + account of personal mode there.
type SignupHook interface {
	OnUserCreated(ctx context.Context, tx pgx.Tx, userID uuid.UUID, email string) error
}

// SignupHookFunc adapts a function to SignupHook (cmd/api: the hook needs services built after
// identity).
type SignupHookFunc func(ctx context.Context, tx pgx.Tx, userID uuid.UUID, email string) error

// OnUserCreated implements SignupHook.
func (f SignupHookFunc) OnUserCreated(ctx context.Context, tx pgx.Tx, userID uuid.UUID, email string) error {
	return f(ctx, tx, userID, email)
}
