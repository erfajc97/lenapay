package payouts

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/erfajc97/lenapay/apps/api/internal/accounts"
	"github.com/erfajc97/lenapay/apps/api/internal/fees"
	"github.com/erfajc97/lenapay/apps/api/internal/notifications"
	"github.com/erfajc97/lenapay/apps/api/internal/orgs"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
)

// The narrow slices of other features' services that payouts calls (P5). The full services
// satisfy them; tests pass small fakes.

// Accounts finds the paying account (implemented by accounts.Service).
type Accounts interface {
	// Get returns an account of orgID (account.not_found when missing or foreign).
	Get(ctx context.Context, orgID, accountID uuid.UUID) (accounts.Account, error)
}

// Fees quotes the payout fee (implemented by fees.Service).
type Fees interface {
	Quote(ctx context.Context, orgID uuid.UUID, op fees.Operation, amount money.Amount) (fees.Quote, error)
}

// Members lists the org's members, to tell its approvers a payout waits for them (orgs.Service).
type Members interface {
	ListMembers(ctx context.Context, p auth.Principal) ([]orgs.Member, error)
}

// Notifier writes the notification outbox inside the payout transaction (notifications.Service).
type Notifier interface {
	Notify(ctx context.Context, tx pgx.Tx, n notifications.Notification) error
}
