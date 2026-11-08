package kyc

import (
	"fmt"
	"slices"
)

// Case kinds (DB CHECK kyc_cases.kind) and the org kind each one maps to.
const (
	KindCompany = "company"
	KindPerson  = "person"

	orgKindCompany  = "company"
	orgKindPersonal = "personal"
)

// caseKindFor maps an organization kind to its onboarding case kind.
func caseKindFor(orgKind string) (string, bool) {
	switch orgKind {
	case orgKindCompany:
		return KindCompany, true
	case orgKindPersonal:
		return KindPerson, true
	}
	return "", false
}

// orgKindFor is the inverse of caseKindFor (the approval hooks receive the org kind).
func orgKindFor(caseKind string) string {
	if caseKind == KindPerson {
		return orgKindPersonal
	}
	return orgKindCompany
}

// transitionAllowed is THE state machine of a case (R-KYC-1): draft → submitted →
// approved | rejected; editing keeps a draft a draft and reopens a rejected case as draft; a
// rejected case may also be submitted again after fixing documents. Approved is final.
func transitionAllowed(from, to Status) bool {
	switch to {
	case StatusDraft:
		return from == StatusDraft || from == StatusRejected
	case StatusSubmitted:
		return from == StatusDraft || from == StatusRejected
	case StatusApproved, StatusRejected:
		return from == StatusSubmitted
	}
	return false
}

// Document kinds (DB CHECK kyc_documents.kind).
const (
	DocBylaws        = "bylaws"
	DocAFIPConstancy = "afip_constancy"
	DocDNIFront      = "dni_front"
	DocDNIBack       = "dni_back"
	DocSelfie        = "selfie"
	DocOther         = "other"
)

// identityDocs are the documents that prove a person's identity (DNI + selfie, R-KYC-1/2).
func identityDocs() []string { return []string{DocDNIFront, DocDNIBack, DocSelfie} }

func validDocKind(kind string) bool {
	return slices.Contains([]string{DocBylaws, DocAFIPConstancy, DocDNIFront, DocDNIBack, DocSelfie, DocOther}, kind)
}

// caseData is the JSON stored in kyc_cases.data: exactly one of the two, matching the case kind.
type caseData struct {
	Company *CompanyData `json:"company,omitempty"`
	Person  *PersonData  `json:"person,omitempty"`
}

// missingItems lists what a case still needs before it can be submitted. Items are
// enums and indexes only (never a DNI: details carry no PII, A7).
func missingItems(kind string, data caseData, docs []Document) []string {
	var missing []string
	has := func(docKind string, repDNI *string) bool {
		return slices.ContainsFunc(docs, func(d Document) bool {
			return d.Kind == docKind && equalPtr(d.RepresentativeDNI, repDNI)
		})
	}
	switch kind {
	case KindPerson:
		if data.Person == nil {
			missing = append(missing, "person")
		}
		for _, k := range identityDocs() {
			if !has(k, nil) {
				missing = append(missing, k)
			}
		}
	case KindCompany:
		if data.Company == nil {
			return append(missing, "company", DocBylaws, DocAFIPConstancy)
		}
		for _, k := range []string{DocBylaws, DocAFIPConstancy} {
			if !has(k, nil) {
				missing = append(missing, k)
			}
		}
		for i, rep := range data.Company.Representatives {
			for _, k := range identityDocs() {
				if !has(k, &rep.DNI) {
					missing = append(missing, fmt.Sprintf("representatives/%d/%s", i, k))
				}
			}
		}
	}
	return missing
}

func equalPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
