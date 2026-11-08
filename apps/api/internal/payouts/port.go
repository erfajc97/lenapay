package payouts

import (
	"context"
	"io"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/erfajc97/lenapay/apps/api/internal/accounts"
	"github.com/erfajc97/lenapay/apps/api/internal/fees"
	"github.com/erfajc97/lenapay/apps/api/internal/notifications"
	"github.com/erfajc97/lenapay/apps/api/internal/orgs"
	"github.com/erfajc97/lenapay/apps/api/internal/payers"
	"github.com/erfajc97/lenapay/apps/api/internal/payins"
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

// DocumentStore keeps payout attachments in private storage (A13); keys are built by the
// service as org/<orgId>/payouts/<payoutId>/<uuid>. provider/fake keeps them on local disk.
type DocumentStore interface {
	Put(ctx context.Context, key, contentType string, content io.Reader) error
}

// Notifier writes the notification outbox inside the payout transaction (notifications.Service).
type Notifier interface {
	Notify(ctx context.Context, tx pgx.Tx, n notifications.Notification) error
}

// CVUs finds Lenapay's own CVUs (payers.Service): a payout to one of them is on-us.
type CVUs interface {
	// OnUsDestination finds an active Lenapay CVU by number or alias (found = false otherwise).
	OnUsDestination(ctx context.Context, to string) (payers.OnUs, bool, error)
	// CVUByNumber re-reads the destination CVU at settlement (it may have been disabled).
	CVUByNumber(ctx context.Context, number string) (payers.CVU, error)
	// Holder is how the paying org shows to the receiver (name and CUIT/DNI).
	Holder(ctx context.Context, orgID uuid.UUID) (name, doc string, err error)
}

// Credits records the receiving side of an on-us payout in its settlement transaction
// (payins.Service).
type Credits interface {
	CreditInternal(ctx context.Context, tx pgx.Tx, in payins.InternalCredit) error
}
