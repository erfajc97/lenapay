package reconciliation

import (
	"time"

	"github.com/google/uuid"

	"lenapay/apps/api/internal/platform/money"
	"lenapay/apps/api/internal/psp"
)

// Status of a run (DB CHECK reconciliation_runs.status).
type Status string

// Run statuses.
const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

// ItemStatus is the outcome of one movement (DB CHECK reconciliation_items.status).
type ItemStatus string

// Item statuses (R-REC-1).
const (
	ItemMatched         ItemStatus = "matched"
	ItemMissingInLedger ItemStatus = "missing_in_ledger"
	ItemMissingAtPSP    ItemStatus = "missing_at_psp"
)

// Run is one reconciliation of a currency and day. OrgID nil is the internal ops run over the
// whole master account; otherwise the run covers only that org's movements.
type Run struct {
	ID                   uuid.UUID
	OrgID                *uuid.UUID
	Currency             money.Currency
	Day                  time.Time
	Status               Status
	MatchedCount         int32
	MissingInLedgerCount int32
	MissingAtPSPCount    int32
	PSPTotal             money.Amount
	LedgerTotal          money.Amount
	Difference           money.Amount
	CreatedAt            time.Time
	FinishedAt           *time.Time
}

// RunInput requests a run (OrgID nil: the whole master account).
type RunInput struct {
	OrgID    *uuid.UUID
	Currency money.Currency
	Day      time.Time
}

// LedgerMovement is one of our records of money crossing the master account: a credited
// payin (in, gross) or a settled payout (out, amount). Keys are every id the PSP may print
// for it (its transfer id and, for payouts, our attempt reference).
type LedgerMovement struct {
	Keys      []string
	Direction psp.Direction
	Amount    money.Amount
	OrgID     uuid.UUID
}

// Item is the outcome of one movement.
type Item struct {
	ProviderTransferID string
	Direction          psp.Direction
	Currency           money.Currency
	PSPAmount          *money.Amount
	LedgerAmount       *money.Amount
	Status             ItemStatus
	OrgID              *uuid.UUID
}

// StoredItem is an item as recorded by a run.
type StoredItem struct {
	Item
	ID        uuid.UUID
	CreatedAt time.Time
}

// Totals summarize items: signed sums (in +, out −) on each side and their difference.
type Totals struct {
	Matched, MissingInLedger, MissingAtPSP int32
	PSP, Ledger, Difference                money.Amount
}
