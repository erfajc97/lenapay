package ledger

import (
	"errors"

	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
)

// Errors that are caller bugs or facts the caller must handle (errors.Is). Business failures
// (insufficient funds, unknown account, inactive hold, cross-currency entry) are apperr codes.
var (
	// ErrInvalidEntry: fewer than two legs, a zero leg, an unknown kind or a missing reference.
	ErrInvalidEntry = errors.New("ledger: invalid entry")
	// ErrUnbalanced: the legs do not sum to zero in every currency.
	ErrUnbalanced = errors.New("ledger: entry does not sum to zero per currency")
	// ErrDuplicateRef: the business fact (ref type, ref id[, kind]) already posted or held
	// (MO3). When it comes from a unique violation the transaction is aborted: roll it back.
	ErrDuplicateRef = errors.New("ledger: the business fact already has a ledger record")
	// ErrCaptureMismatch: a capture entry does not debit exactly the held amount on the held
	// account and currency.
	ErrCaptureMismatch = errors.New("ledger: capture entry does not consume the hold exactly")
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

// SystemCodes lists every system account (DB CHECK accounts.system_code).
func SystemCodes() []SystemCode {
	return []SystemCode{SystemPSPMaster, SystemFeesRevenue, SystemPayoutClearing, SystemSuspense}
}

// Currencies are the currencies every account holds a ledger account in (DB CHECK).
func Currencies() []money.Currency {
	return []money.Currency{money.ARS, money.USD}
}

func (k Kind) valid() bool {
	switch k {
	case KindPayin, KindPayout, KindInternalTransfer, KindReversal, KindAdjustment:
		return true
	}
	return false
}

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

// Balance is the state of one account in one currency. Available = Posted − Held.
type Balance struct {
	AccountID uuid.UUID
	Posted    money.Amount
	Held      money.Amount
	Available money.Amount
}

func newBalance(accountID uuid.UUID, currency string, posted, held int64) Balance {
	c := money.Currency(currency)
	return Balance{
		AccountID: accountID,
		Posted:    money.New(posted, c),
		Held:      money.New(held, c),
		// held >= 0 and posted >= held for customer accounts (DB CHECK), so no overflow here;
		// system accounts are bounded by the int64 checks of every posting.
		Available: money.New(posted-held, c),
	}
}
