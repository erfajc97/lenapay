package payouts

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/money"
	"lenapay/apps/api/internal/psp"
)

func TestClassifyDestination(t *testing.T) {
	t.Parallel()
	cases := []struct {
		to   string
		want psp.DestinationType
		ok   bool
	}{
		{"0000003100010000000001", psp.DestinationCVU, true},
		{"0170099220000067797370", psp.DestinationCBU, true},
		{"proveedor.andes", psp.DestinationAlias, true},
		{"abc-de", psp.DestinationAlias, true},               // 6 chars: the shortest alias
		{"abcde", "", false},                                 // 5 chars
		{"a2345678901234567890", psp.DestinationAlias, true}, // 20 chars: the longest alias
		{"a23456789012345678901", "", false},                 // 21 chars, not 22 digits
		{"017009922000006779737", "", false},                 // 21 digits
		{"mi alias", "", false},
	}
	for _, c := range cases {
		got, err := ClassifyDestination(c.to)
		if c.ok != (err == nil) || got != c.want {
			t.Errorf("ClassifyDestination(%q) = %q, %v; want %q ok=%v", c.to, got, err, c.want, c.ok)
		}
		if !c.ok && !apperr.IsCode(err, apperr.CodeDestinationInvalid) {
			t.Errorf("ClassifyDestination(%q) err = %v, want destination.invalid", c.to, err)
		}
	}
}

func TestRequireMoney(t *testing.T) {
	t.Parallel()
	ok := auth.Principal{UserID: uuid.New(), OrgID: uuid.New(), Role: auth.RoleApprover, KYCStatus: auth.KYCApproved, StepUp: true}
	if err := requireMoney(ok, auth.RoleApprover, true); err != nil {
		t.Fatalf("base principal must pass: %v", err)
	}
	noStepUp, kyc, viewer, noOrg := ok, ok, ok, ok
	noStepUp.StepUp = false
	kyc.KYCStatus = auth.KYCSubmitted
	viewer.Role = auth.RoleOperator
	noOrg.OrgID = uuid.Nil
	for _, c := range []struct {
		name string
		p    auth.Principal
		code apperr.Code
	}{
		{"no step-up", noStepUp, apperr.CodeStepUpRequired},
		{"kyc not approved", kyc, apperr.CodeKYCNotApproved},
		{"role below approver", viewer, apperr.CodeForbidden},
		{"no org", noOrg, apperr.CodeOrgNotFound},
	} {
		if err := requireMoney(c.p, auth.RoleApprover, true); !apperr.IsCode(err, c.code) {
			t.Errorf("%s: err = %v, want %s", c.name, err, c.code)
		}
	}
	if err := requireMoney(noStepUp, auth.RoleApprover, false); err != nil {
		t.Errorf("step-up not asked: %v", err)
	}
}

func TestValidateCreateCaps(t *testing.T) {
	t.Parallel()
	key16 := "0123456789abcdef"
	concept140 := string(bytes.Repeat([]byte("ñ"), 70)) + string(bytes.Repeat([]byte("a"), 70))
	if err := validateCreate(key16, concept140); err != nil {
		t.Fatalf("last passing values must pass: %v", err)
	}
	for name, c := range map[string][2]string{
		"key 15":      {key16[:15], "ok"},
		"key 129":     {string(bytes.Repeat([]byte("k"), 129)), "ok"},
		"concept 0":   {key16, ""},
		"concept 141": {key16, concept140 + "a"},
	} {
		if err := validateCreate(c[0], c[1]); !apperr.IsCode(err, apperr.CodeValidation) {
			t.Errorf("%s: err = %v, want validation.failed", name, err)
		}
	}
}

func TestSettlementEntrySumsToZero(t *testing.T) {
	t.Parallel()
	master, revenue := uuid.New(), uuid.New()
	po := Payout{ID: uuid.New(), OrgID: uuid.New(), AccountID: uuid.New(), Amount: money.New(1_000_000, money.ARS), Fee: money.New(17_000, money.ARS)}
	e, err := settlementEntry(po, master, revenue, "k")
	if err != nil {
		t.Fatal(err)
	}
	want := map[uuid.UUID]int64{po.AccountID: -1_017_000, master: 1_000_000, revenue: 17_000}
	if len(e.Legs) != 3 {
		t.Fatalf("legs = %+v", e.Legs)
	}
	for _, l := range e.Legs {
		if want[l.AccountID] != l.Amount.Minor || l.Amount.Currency != money.ARS {
			t.Errorf("leg %s = %d, want %d", l.AccountID, l.Amount.Minor, want[l.AccountID])
		}
	}
	po.Fee = money.Zero(money.USD)
	po.Amount = money.New(500, money.USD)
	e, err = settlementEntry(po, master, revenue, "k")
	if err != nil || len(e.Legs) != 2 {
		t.Fatalf("zero fee must have no fee leg: %+v, %v", e.Legs, err)
	}
}

func TestFormatMoney(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		a    money.Amount
		want string
	}{
		{money.New(150_000_00, money.ARS), "$ 150.000,00"},
		{money.New(5, money.ARS), "$ 0,05"},
		{money.New(1_017_000, money.ARS), "$ 10.170,00"},
		{money.New(123_456_789, money.USD), "US$ 1.234.567,89"},
		{money.New(-99_999, money.ARS), "-$ 999,99"},
	} {
		if got := formatMoney(c.a); got != c.want {
			t.Errorf("formatMoney(%v) = %q, want %q", c.a, got, c.want)
		}
	}
}

func TestPDFTextEscapes(t *testing.T) {
	t.Parallel()
	if got := pdfText(`Pago (obra) \ 1`); got != `Pago \(obra\) \\ 1` {
		t.Errorf("delimiters: %q", got)
	}
	if got := pdfText("Comisión"); got != `Comisi\363n` {
		t.Errorf("latin-1: %q", got)
	}
	if got := pdfText("€"); got != "?" {
		t.Errorf("outside latin-1: %q", got)
	}
}

// The proof is a structurally valid PDF: every xref offset points at its object.
func TestRenderProofIsAValidPDF(t *testing.T) {
	t.Parallel()
	settled := time.Date(2026, 11, 7, 18, 30, 0, 0, time.UTC)
	ref := "bind-tr-1"
	po := Payout{
		ID: uuid.New(), Amount: money.New(1_000_000, money.ARS), Fee: money.New(17_000, money.ARS),
		Destination: psp.Destination{
			Type: psp.DestinationAlias, Value: "proveedor.andes", AccountNumber: "0170099220000067797370",
			HolderName: "Proveedora Andes SRL", HolderCUIT: "30711122233", BankName: "Banco Galicia",
		},
		Concept: "Certificado (obra) 12", Status: StatusSettled, SettledAt: &settled, ProviderTransferID: &ref,
	}
	loc := time.FixedZone("ART", -3*3600)
	pdf, err := renderProof(po, loc)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"%PDF-1.4", "(Lenapay)", "($ 10.170,00)", "(30-71112223-3)", "(07/11/2026 15:30)", `(Certificado \(obra\) 12)`, "(proveedor.andes)", "%%EOF"} {
		if !bytes.Contains(pdf, []byte(want)) {
			t.Errorf("proof lacks %q", want)
		}
	}
	m := regexp.MustCompile(`startxref\n(\d+)`).FindSubmatch(pdf)
	if m == nil {
		t.Fatal("no startxref")
	}
	xref, _ := strconv.Atoi(string(m[1]))
	if !bytes.HasPrefix(pdf[xref:], []byte("xref\n")) {
		t.Fatalf("startxref %d does not point at the xref table", xref)
	}
	offsets := regexp.MustCompile(`(\d{10}) 00000 n `).FindAllSubmatch(pdf[xref:], -1)
	if len(offsets) != 6 {
		t.Fatalf("%d objects in xref, want 6", len(offsets))
	}
	for i, o := range offsets {
		off, _ := strconv.Atoi(string(o[1]))
		if !bytes.HasPrefix(pdf[off:], fmt.Appendf(nil, "%d 0 obj", i+1)) {
			t.Errorf("object %d not at offset %d", i+1, off)
		}
	}
}
