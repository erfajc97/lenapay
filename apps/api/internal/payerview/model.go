package payerview

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"lenapay/apps/api/internal/erplink"
	"lenapay/apps/api/internal/platform/money"
)

// Link statuses (DB CHECK payer_links.status): a hidden company stays linked but out of the list.
const (
	statusActive = "active"
	statusHidden = "hidden"
)

// Caps shared with the contract (PayMyPaymentCompanyRequest) and payouts (G12).
const (
	maxConcept     = 140 // payouts.concept
	maxUserConcept = 120 // what the person may type; the installment prefix needs the rest
	maxInstallment = 999
)

// person is the caller as a verified person: their personal org and its verified DNI.
type person struct {
	orgID uuid.UUID
	dni   string
}

// CVU is one of the person's collection accounts at a company.
type CVU struct {
	Currency money.Currency
	Number   string
	Alias    *string
}

// Company is a company the person pays to, as the person may see it.
type Company struct {
	LinkID      uuid.UUID
	CompanyName string
	// ProjectName is the project account the person pays into (nil for the main account).
	ProjectName   *string
	CVUs          []CVU
	TotalPaid     []money.Amount // credited payins, one per currency
	LastPaymentAt *time.Time
	ERPLinked     bool
	Hidden        bool
}

// Payin is one of the person's payins to a company.
type Payin struct {
	ID         uuid.UUID
	CVU        string
	Amount     money.Amount
	Status     string // received | credited | failed (payins.status)
	CreatedAt  time.Time
	CreditedAt *time.Time
}

// Installments is the person's plan at a company (empty when the company is not ERP-linked).
type Installments struct {
	ERPLinked bool
	Items     []erplink.Installment
}

// PayInput pays a company from the personal account.
type PayInput struct {
	Amount            money.Amount
	InstallmentNumber *int
	Concept           *string
	IdempotencyKey    string // required (MO2), checked by payouts
}

// dni8 is the 8-digit matching key of a verified DNI (left-padded, as a CUIT/CUIL carries it);
// false when the document cannot be a DNI (not 7–8 digits).
func dni8(dni string) (string, bool) {
	if n := len(dni); n < 7 || n > 8 || strings.Trim(dni, "0123456789") != "" {
		return "", false
	}
	return strings.Repeat("0", 8-len(dni)) + dni, true
}
