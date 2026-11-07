package kyc

import (
	"slices"
	"testing"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
)

// The whole state machine (R-KYC-1): every pair of statuses, so a new transition cannot
// slip in unnoticed. Approved is final; only a submitted case is decided.
func TestTransitionAllowed(t *testing.T) {
	t.Parallel()
	all := []Status{StatusDraft, StatusSubmitted, StatusApproved, StatusRejected}
	allowed := map[[2]Status]bool{
		{StatusDraft, StatusDraft}:        true,
		{StatusRejected, StatusDraft}:     true,
		{StatusDraft, StatusSubmitted}:    true,
		{StatusRejected, StatusSubmitted}: true,
		{StatusSubmitted, StatusApproved}: true,
		{StatusSubmitted, StatusRejected}: true,
	}
	for _, from := range all {
		for _, to := range all {
			if got := transitionAllowed(from, to); got != allowed[[2]Status{from, to}] {
				t.Errorf("%s → %s = %v", from, to, got)
			}
		}
	}
	if transitionAllowed(StatusDraft, Status("archived")) {
		t.Error("unknown target status allowed")
	}
}

func TestMissingItems(t *testing.T) {
	t.Parallel()
	dni := "20111222"
	other := "30111222"
	doc := func(kind string, rep *string) Document { return Document{Kind: kind, RepresentativeDNI: rep} }
	person := &PersonData{FullName: "Ana Pérez", DNI: "25111222"}
	company := &CompanyData{CUIT: "30712345678", Representatives: []Representative{{FullName: "Rep", DNI: dni}}}
	fullCompanyDocs := []Document{doc(DocBylaws, nil), doc(DocAFIPConstancy, nil), doc(DocDNIFront, &dni), doc(DocDNIBack, &dni), doc(DocSelfie, &dni)}
	tests := []struct {
		name string
		kind string
		data caseData
		docs []Document
		want []string
	}{
		{"complete person", KindPerson, caseData{Person: person}, []Document{doc(DocDNIFront, nil), doc(DocDNIBack, nil), doc(DocSelfie, nil)}, nil},
		{"person without data or selfie", KindPerson, caseData{}, []Document{doc(DocDNIFront, nil), doc(DocDNIBack, nil)}, []string{"person", DocSelfie}},
		{"complete company", KindCompany, caseData{Company: company}, fullCompanyDocs, nil},
		{"company without data", KindCompany, caseData{}, nil, []string{"company", DocBylaws, DocAFIPConstancy}},
		{
			"selfie of another representative does not count", KindCompany,
			caseData{Company: company},
			append(slices.Clone(fullCompanyDocs[:4]), doc(DocSelfie, &other)),
			[]string{"representatives/0/selfie"},
		},
		{
			"company identity docs without representative do not count", KindCompany,
			caseData{Company: company},
			[]Document{doc(DocBylaws, nil), doc(DocAFIPConstancy, nil), doc(DocDNIFront, nil), doc(DocDNIBack, nil), doc(DocSelfie, nil)},
			[]string{"representatives/0/dni_front", "representatives/0/dni_back", "representatives/0/selfie"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := missingItems(tt.kind, tt.data, tt.docs); !slices.Equal(got, tt.want) {
				t.Fatalf("missing %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckRepresentative(t *testing.T) {
	t.Parallel()
	dni := "20111222"
	tests := []struct {
		name string
		kind string
		in   DocumentUpload
		ok   bool
	}{
		{"company bylaws", KindCompany, DocumentUpload{Kind: DocBylaws}, true},
		{"company bylaws with a representative", KindCompany, DocumentUpload{Kind: DocBylaws, RepresentativeDNI: &dni}, false},
		{"company selfie of a representative", KindCompany, DocumentUpload{Kind: DocSelfie, RepresentativeDNI: &dni}, true},
		{"company selfie without representative", KindCompany, DocumentUpload{Kind: DocSelfie}, false},
		{"person selfie", KindPerson, DocumentUpload{Kind: DocSelfie}, true},
		{"person with a representative", KindPerson, DocumentUpload{Kind: DocDNIFront, RepresentativeDNI: &dni}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := checkRepresentative(tt.kind, tt.in)
			if (err == nil) != tt.ok || (err != nil && !apperr.IsCode(err, apperr.CodeValidation)) {
				t.Fatalf("checkRepresentative = %v, want ok=%v", err, tt.ok)
			}
		})
	}
}

// Exactly the data block of the case kind (UpdateKycRequest: "exactly one of").
func TestDataFor(t *testing.T) {
	t.Parallel()
	company, person := &CompanyData{CUIT: "30712345678"}, &PersonData{DNI: "25111222"}
	tests := []struct {
		name  string
		kind  string
		in    UpdateInput
		field string
	}{
		{"company data on a company", KindCompany, UpdateInput{Company: company}, ""},
		{"person data on a person", KindPerson, UpdateInput{Person: person}, ""},
		{"both on a company", KindCompany, UpdateInput{Company: company, Person: person}, "/company"},
		{"person data on a company", KindCompany, UpdateInput{Person: person}, "/company"},
		{"company data on a person", KindPerson, UpdateInput{Company: company}, "/person"},
		{"nothing on a person", KindPerson, UpdateInput{}, "/person"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := dataFor(tt.kind, tt.in)
			if tt.field == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var appErr *apperr.Error
			if !apperr.IsCode(err, apperr.CodeValidation) || !asAppErr(err, &appErr) || appErr.Details["field"] != tt.field {
				t.Fatalf("dataFor = %v, want validation on %s", err, tt.field)
			}
		})
	}
}

func asAppErr(err error, target **apperr.Error) bool {
	e, ok := err.(*apperr.Error) //nolint:errorlint // dataFor returns the *apperr.Error itself
	*target = e
	return ok
}

func TestKinds(t *testing.T) {
	t.Parallel()
	if k, ok := caseKindFor("company"); !ok || k != KindCompany || orgKindFor(k) != "company" {
		t.Fatal("company mapping")
	}
	if k, ok := caseKindFor("personal"); !ok || k != KindPerson || orgKindFor(k) != "personal" {
		t.Fatal("personal mapping")
	}
	if _, ok := caseKindFor("trust"); ok {
		t.Fatal("unknown org kind accepted")
	}
}
