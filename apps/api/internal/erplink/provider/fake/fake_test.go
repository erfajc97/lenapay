package fake_test

import (
	"regexp"
	"testing"
	"time"

	"lenapay/apps/api/internal/erplink"
	"lenapay/apps/api/internal/erplink/provider/fake"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/config"
	"lenapay/apps/api/internal/platform/money"
)

// The real fixtures of apps/api/fixtures/erp, so a malformed fixture fails here first.
const fixtures = "../../../../fixtures/erp"

func TestFixturesFeedTheIdentityIndex(t *testing.T) {
	t.Parallel()
	d := fake.NewDirectory(fixtures)
	entries, err := d.IdentityIndex(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	docNumber := regexp.MustCompile(`^[0-9]{7,11}$`) // the CHECK of erp_identity_index.doc_number
	seen := map[string]bool{}
	orgs := 0
	var dds string
	for _, e := range entries {
		key := e.DocType + ":" + e.DocNumber
		if seen[key] {
			t.Errorf("duplicate identity %s", key)
		}
		seen[key] = true
		if !docNumber.MatchString(e.DocNumber) || (e.DocType != "cuit" && e.DocType != "cuil" && e.DocType != "dni") || e.DisplayName == "" {
			t.Errorf("invalid entry %+v", e)
		}
		if e.ERPClientID == nil {
			orgs++
		}
		if e.DocNumber == "30712345678" {
			dds = e.DisplayName
		}
	}
	if orgs != 10 || dds != "Desarrollos Del Sur SA" {
		t.Fatalf("orgs = %d, Desarrollos Del Sur = %q", orgs, dds)
	}
}

func TestVerifyCredentials(t *testing.T) {
	t.Parallel()
	d := fake.NewDirectory(fixtures)
	ctx := t.Context()
	if err := d.VerifyCredentials(ctx, "erp-org-001", " Admin@DesarrollosDelSur.com.ar ", config.Secret(fake.DevPassword)); err != nil {
		t.Fatalf("known user with the dev password: %v", err)
	}
	for name, c := range map[string][3]string{
		"wrong password":   {"erp-org-001", "admin@desarrollosdelsur.com.ar", "lebane1234"},
		"user of another":  {"erp-org-002", "admin@desarrollosdelsur.com.ar", fake.DevPassword},
		"unknown erp org":  {"erp-org-999", "admin@desarrollosdelsur.com.ar", fake.DevPassword},
		"unknown username": {"erp-org-001", "intruso@example.com", fake.DevPassword},
	} {
		if err := d.VerifyCredentials(ctx, c[0], c[1], config.Secret(c[2])); !apperr.IsCode(err, apperr.CodeERPCredentialsInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestProjectsAndBuyers(t *testing.T) {
	t.Parallel()
	d := fake.NewDirectory(fixtures)
	projects, err := d.Projects(t.Context(), "erp-org-001")
	if err != nil || len(projects) != 3 || projects[0].Name != "Torre Palermo Soho" {
		t.Fatalf("projects = %+v, %v", projects, err)
	}
	buyers, err := d.Buyers(t.Context(), "erp-org-001")
	if err != nil || len(buyers) != 5 {
		t.Fatalf("buyers = %+v, %v", buyers, err)
	}
	refs := map[string]bool{}
	for _, p := range projects {
		refs[p.Ref] = true
	}
	for _, b := range buyers {
		if !refs[b.ProjectRef] {
			t.Errorf("buyer %s points at unknown project %s", b.Ref, b.ProjectRef)
		}
	}
	if _, err := d.Projects(t.Context(), "erp-org-999"); !apperr.IsCode(err, apperr.CodeERPLinkNotFound) {
		t.Fatalf("unknown org err = %v", err)
	}
}

func TestInstallmentsOfEveryFixtureBuyerParse(t *testing.T) {
	t.Parallel()
	d := fake.NewDirectory(fixtures)
	ctx := t.Context()
	for _, org := range []string{"erp-org-001", "erp-org-002"} {
		buyers, err := d.Buyers(ctx, org)
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range buyers {
			plan, err := d.Installments(ctx, org, b.Ref)
			if err != nil || len(plan) == 0 {
				t.Fatalf("%s: plan = %+v, %v", b.Ref, plan, err)
			}
			for i, in := range plan {
				if in.Number != i+1 || !in.Amount.IsPositive() || in.DueDate.IsZero() {
					t.Errorf("%s: installment %d = %+v", b.Ref, i, in)
				}
			}
		}
	}
}

func TestInstallmentsOfThePersonaBuyer(t *testing.T) {
	t.Parallel()
	d := fake.NewDirectory(fixtures)
	plan, err := d.Installments(t.Context(), "erp-org-002", "erp-org-002-buyer-005")
	if err != nil || len(plan) != 12 {
		t.Fatalf("plan = %d, %v", len(plan), err)
	}
	first, overdue := plan[0], plan[3]
	if first.Amount != money.New(85_000_000, money.ARS) || first.Status != erplink.InstallmentPaid ||
		first.DueDate.Format(time.DateOnly) != "2026-07-10" || first.AdjustmentIndex == nil || *first.AdjustmentIndex != "CAC" {
		t.Fatalf("first = %+v", first)
	}
	if overdue.Status != erplink.InstallmentOverdue || plan[11].Status != erplink.InstallmentPending {
		t.Fatalf("statuses = %s, %s", overdue.Status, plan[11].Status)
	}
	none, err := d.Installments(t.Context(), "erp-org-002", "erp-org-002-buyer-999")
	if err != nil || len(none) != 0 {
		t.Fatalf("unknown buyer = %+v, %v", none, err)
	}
	if _, err := d.Installments(t.Context(), "erp-org-999", "x"); !apperr.IsCode(err, apperr.CodeERPLinkNotFound) {
		t.Fatalf("unknown org err = %v", err)
	}
}
