package fake_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/erfajc97/lenapay/apps/api/internal/kyc"
	"github.com/erfajc97/lenapay/apps/api/internal/kyc/provider/fake"
)

// R-KYC-2: the fake approves valid-looking data and rejects the configured DNI list.
func TestVerifyPerson(t *testing.T) {
	t.Parallel()
	p := fake.NewProvider([]string{" 11111111 ", ""})
	ok := kyc.PersonCheck{Reference: "case-1", DNI: "25111222", FullName: "Ana Pérez", SelfieKey: "org/x/kyc/y"}
	tests := []struct {
		name     string
		mutate   func(c *kyc.PersonCheck)
		approved bool
		reason   string
	}{
		{"valid data", func(*kyc.PersonCheck) {}, true, ""},
		{"rejected DNI", func(c *kyc.PersonCheck) { c.DNI = "11111111" }, false, fake.ReasonDNIRejected},
		{"malformed DNI", func(c *kyc.PersonCheck) { c.DNI = "123" }, false, fake.ReasonInvalidData},
		{"no selfie", func(c *kyc.PersonCheck) { c.SelfieKey = "" }, false, fake.ReasonInvalidData},
		{"blank name", func(c *kyc.PersonCheck) { c.FullName = "  " }, false, fake.ReasonInvalidData},
	}
	for i, tt := range tests {
		c := ok
		c.Reference = c.Reference + strings.Repeat("x", i)
		tt.mutate(&c)
		v, err := p.VerifyPerson(t.Context(), c)
		if err != nil || v.Approved != tt.approved || v.Reason != tt.reason || !strings.HasPrefix(v.ProviderRef, "fake-") {
			t.Errorf("%s: verdict %+v err %v", tt.name, v, err)
		}
	}
}

// Idempotent by reference: the same reference replays the first verdict, like the vendor.
func TestVerifyIsIdempotentByReference(t *testing.T) {
	t.Parallel()
	p := fake.NewProvider([]string{"11111111"})
	first, _ := p.VerifyPerson(t.Context(), kyc.PersonCheck{Reference: "r", DNI: "11111111", FullName: "A", SelfieKey: "k"})
	again, _ := p.VerifyPerson(t.Context(), kyc.PersonCheck{Reference: "r", DNI: "25111222", FullName: "A", SelfieKey: "k"})
	if again != first {
		t.Fatalf("replay %+v, want %+v", again, first)
	}
}

func TestVerifyCompanyAndOutage(t *testing.T) {
	t.Parallel()
	p := fake.NewProvider(nil)
	v, err := p.VerifyCompany(t.Context(), kyc.CompanyCheck{Reference: "c1", CUIT: "30712345678", LegalName: "Constructora Sur"})
	if err != nil || !v.Approved {
		t.Fatalf("valid company %+v %v", v, err)
	}
	v, _ = p.VerifyCompany(t.Context(), kyc.CompanyCheck{Reference: "c2", CUIT: "3071234567", LegalName: "Constructora Sur"})
	if v.Approved || v.Reason != fake.ReasonInvalidData {
		t.Fatalf("short CUIT %+v", v)
	}
	p.FailNext(1)
	if _, err := p.VerifyCompany(t.Context(), kyc.CompanyCheck{Reference: "c3", CUIT: "30712345678", LegalName: "X"}); !errors.Is(err, fake.ErrUnavailable) {
		t.Fatalf("outage: %v", err)
	}
	if v, err := p.VerifyCompany(t.Context(), kyc.CompanyCheck{Reference: "c3", CUIT: "30712345678", LegalName: "X"}); err != nil || !v.Approved {
		t.Fatalf("after the outage %+v %v", v, err)
	}
}

func TestDocumentStore(t *testing.T) {
	t.Parallel()
	s := fake.NewDocumentStore()
	if err := s.Put(t.Context(), "org/a/kyc/b", "application/pdf", strings.NewReader("%PDF-1")); err != nil {
		t.Fatal(err)
	}
	if b, ok := s.Object("org/a/kyc/b"); !ok || string(b) != "%PDF-1" {
		t.Fatalf("object %q %v", b, ok)
	}
}
