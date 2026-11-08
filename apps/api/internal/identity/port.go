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

// Accounts checks the account a user prefers as default (implemented by accounts.Service through
// a cmd/api adapter): archived reports an archived account; a missing or foreign one is
// account.not_found.
type Accounts interface {
	AccountArchived(ctx context.Context, orgID, accountID uuid.UUID) (archived bool, err error)
}

// AccountsFunc adapts a function to Accounts (identity is built before accounts).
type AccountsFunc func(ctx context.Context, orgID, accountID uuid.UUID) (bool, error)

// AccountArchived implements Accounts.
func (f AccountsFunc) AccountArchived(ctx context.Context, orgID, accountID uuid.UUID) (bool, error) {
	return f(ctx, orgID, accountID)
}

// InviteAcceptor accepts the pending invites of the ERP users import for a user whose email is
// verified, inside the caller's transaction (implemented by orgs.Service).
type InviteAcceptor interface {
	AcceptERPInvites(ctx context.Context, tx pgx.Tx, userID uuid.UUID, email string) (accepted int, err error)
}

// InviteAcceptorFunc adapts a function to InviteAcceptor (orgs is built after identity).
type InviteAcceptorFunc func(ctx context.Context, tx pgx.Tx, userID uuid.UUID, email string) (int, error)

// AcceptERPInvites implements InviteAcceptor.
func (f InviteAcceptorFunc) AcceptERPInvites(ctx context.Context, tx pgx.Tx, userID uuid.UUID, email string) (int, error) {
	return f(ctx, tx, userID, email)
}
