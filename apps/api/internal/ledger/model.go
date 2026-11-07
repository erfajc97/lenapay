package ledger

import (
	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
)

// SystemCode names a Lenapay-owned account (accounts.system_code).
type SystemCode string

// System accounts (one ledger account per currency each).
const (
	SystemPSPMaster      SystemCode = "psp_master"
	SystemFeesRevenue    SystemCode = "fees_revenue"
	SystemPayoutClearing SystemCode = "payout_clearing"
	SystemSuspense       SystemCode = "suspense"
)

// Kind of a ledger transaction (DB CHECK ledger_transactions.kind).
type Kind string

// Transaction kinds.
const (
	KindPayin            Kind = "payin"
	KindPayout           Kind = "payout"
	KindInternalTransfer Kind = "internal_transfer"
	KindReversal         Kind = "reversal"
	KindAdjustment       Kind = "adjustment"
)

// Leg moves Amount (signed: + credits, − debits) on the ledger account of (AccountID,
// Amount.Currency). The legs of an Entry sum to zero per currency.
type Leg struct {
	AccountID uuid.UUID
	Amount    money.Amount
}

// Entry is one balanced ledger transaction about a business fact (RefType, RefID).
type Entry struct {
	Kind           Kind
	RefType        string // e.g. "payin", "payout", "internal_transfer"
	RefID          uuid.UUID
	OrgID          *uuid.UUID
	IdempotencyKey string
	Memo           string
	CreatedBy      *uuid.UUID
	Legs           []Leg
}

// HoldInput reserves Amount on an account (available = posted − held) for a business fact.
type HoldInput struct {
	AccountID uuid.UUID
	Amount    money.Amount
	RefType   string
	RefID     uuid.UUID
}

// Balance is the state of one account in one currency.
type Balance struct {
	AccountID uuid.UUID
	Posted    money.Amount
	Held      money.Amount
	Available money.Amount
}
