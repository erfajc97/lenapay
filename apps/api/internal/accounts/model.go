package accounts

import (
	"time"

	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
)

// Kind of an org account.
type Kind string

// Account kinds (DB CHECK accounts.kind; system accounts belong to ledger).
const (
	KindMain     Kind = "main"
	KindProject  Kind = "project"
	KindPersonal Kind = "personal"
)

// CurrencyBalance is one currency of an account.
type CurrencyBalance struct {
	Posted    money.Amount
	Held      money.Amount
	Available money.Amount
}

// Account is an org account with its balances.
type Account struct {
	ID            uuid.UUID
	OrgID         uuid.UUID
	Kind          Kind
	Name          string
	Archived      bool
	ERPProjectRef *string
	Balances      []CurrencyBalance
	CreatedAt     time.Time
}

// CreateInput creates a project account.
type CreateInput struct {
	Name string
}

// UpdateInput renames and/or archives (nil = keep, A4).
type UpdateInput struct {
	Name     *string
	Archived *bool
}

// Summary is the balances screen: totals per currency and per account.
type Summary struct {
	Totals   []CurrencyBalance
	Accounts []Account
}

// TransferInput moves money between two accounts of the same org and currency.
type TransferInput struct {
	FromAccountID  uuid.UUID
	ToAccountID    uuid.UUID
	Amount         money.Amount
	Concept        string
	IdempotencyKey string // required (MO2)
}

// Transfer is a posted internal transfer.
type Transfer struct {
	ID            uuid.UUID
	FromAccountID uuid.UUID
	ToAccountID   uuid.UUID
	Amount        money.Amount
	Concept       *string
	CreatedAt     time.Time
}
