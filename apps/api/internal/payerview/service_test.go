package payerview

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"lenapay/apps/api/internal/erplink"
	"lenapay/apps/api/internal/payerview/store"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/money"
)

func TestDNI8PadsSevenDigitsAndRejectsNonDNIs(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"30123456": "30123456", "8123456": "08123456"} {
		if got, ok := dni8(in); !ok || got != want {
			t.Errorf("dni8(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "812345", "301234567", "27301234568", "3012345a"} {
		if got, ok := dni8(in); ok {
			t.Errorf("dni8(%q) = %q, want no key", in, got)
		}
	}
}

// planERP answers a fixed plan for every org (only BuyerInstallments is used by concept).
type planERP struct{ plan []erplink.Installment }

func (planERP) LinkedOrgs(context.Context, []uuid.UUID) (map[uuid.UUID]bool, error) {
	return map[uuid.UUID]bool{}, nil
}

func (e planERP) BuyerInstallments(context.Context, uuid.UUID, *string) (bool, []erplink.Installment, error) {
	return true, e.plan, nil
}

func TestConceptNamesTheInstallmentAndChecksIt(t *testing.T) {
	t.Parallel()
	svc := &service{deps: Deps{ERP: planERP{plan: []erplink.Installment{
		{Number: 1, DueDate: time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC), Amount: money.New(85_000_000, money.ARS), Status: erplink.InstallmentPaid},
		{Number: 2, DueDate: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC), Amount: money.New(86_250_000, money.ARS), Status: erplink.InstallmentPending},
	}}}}
	link := store.ListLinksRow{CompanyName: "Constructora Andina SRL"}
	num := func(n int) *int { return &n }
	text := func(s string) *string { return &s }
	ars := money.New(100, money.ARS)

	for name, tc := range map[string]struct {
		in   PayInput
		want string
		code apperr.Code
	}{
		"company name by default": {in: PayInput{Amount: ars}, want: "Constructora Andina SRL"},
		"typed concept, trimmed":  {in: PayInput{Amount: ars, Concept: text("  Seña cochera ")}, want: "Seña cochera"},
		"installment prefix":      {in: PayInput{Amount: ars, InstallmentNumber: num(2)}, want: "Cuota 2 - Constructora Andina SRL"},
		"installment + concept":   {in: PayInput{Amount: ars, InstallmentNumber: num(1), Concept: text("Saldo")}, want: "Cuota 1 - Saldo"},
		"unknown installment":     {in: PayInput{Amount: ars, InstallmentNumber: num(3)}, code: apperr.CodeInstallmentNotFound},
		"installment in USD":      {in: PayInput{Amount: money.New(100, money.USD), InstallmentNumber: num(1)}, code: apperr.CodeCurrencyMismatch},
		"installment zero":        {in: PayInput{Amount: ars, InstallmentNumber: num(0)}, code: apperr.CodeValidation},
		"installment 1000":        {in: PayInput{Amount: ars, InstallmentNumber: num(maxInstallment + 1)}, code: apperr.CodeValidation},
		"blank concept":           {in: PayInput{Amount: ars, Concept: text("   ")}, code: apperr.CodeValidation},
		"concept at the cap":      {in: PayInput{Amount: ars, Concept: text(strings.Repeat("ñ", maxUserConcept))}, want: strings.Repeat("ñ", maxUserConcept)},
		"concept over the cap":    {in: PayInput{Amount: ars, Concept: text(strings.Repeat("ñ", maxUserConcept+1))}, code: apperr.CodeValidation},
	} {
		got, err := svc.concept(t.Context(), link, tc.in)
		if tc.code != "" {
			if !apperr.IsCode(err, tc.code) {
				t.Errorf("%s: err = %v, want %s", name, err, tc.code)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("%s: concept = %q, %v; want %q", name, got, err, tc.want)
		}
	}
}

func TestConceptFitsThePayoutCap(t *testing.T) {
	t.Parallel()
	svc := &service{deps: Deps{ERP: planERP{plan: []erplink.Installment{{Number: 999, Amount: money.New(1, money.ARS)}}}}}
	long := strings.Repeat("Constructora ", 20) // a 200-character legal name
	n := 999
	got, err := svc.concept(t.Context(), store.ListLinksRow{CompanyName: long}, PayInput{Amount: money.New(1, money.ARS), InstallmentNumber: &n})
	if err != nil || len([]rune(got)) != maxConcept || !strings.HasPrefix(got, "Cuota 999 - Constructora") {
		t.Fatalf("concept = %q (%d runes), %v", got, len([]rune(got)), err)
	}
}
