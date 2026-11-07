package kyc

import (
	"io"
	"time"

	"github.com/google/uuid"
)

// Status of a case (DB CHECK kyc_cases.status; mirrored on organizations.kyc_status).
type Status string

// Case statuses: draft → submitted → approved | rejected.
const (
	StatusDraft     Status = "draft"
	StatusSubmitted Status = "submitted"
	StatusApproved  Status = "approved"
	StatusRejected  Status = "rejected"
)

// Address is a postal address.
type Address struct {
	Street, Number, Floor, City, Province, PostalCode, Country string
}

// Representative is a company representative (identity checked with DNI + selfie).
type Representative struct {
	FullName, DNI, Position string
}

// CompanyData is the onboarding data of a company (R-KYC-1).
type CompanyData struct {
	CUIT            string
	LegalName       string
	LegalAddress    Address
	Representatives []Representative
}

// PersonData is the onboarding data of a person (R-KYC-2).
type PersonData struct {
	FullName  string
	DNI       string
	BirthDate time.Time
	Address   Address
}

// Case is the onboarding case of one org.
type Case struct {
	ID             uuid.UUID
	OrgID          uuid.UUID
	Kind           string // company | person
	Status         Status
	Company        *CompanyData
	Person         *PersonData
	Documents      []Document
	DecisionReason *string
	SubmittedAt    *time.Time
	DecidedAt      *time.Time
	UpdatedAt      time.Time
}

// UpdateInput replaces the case data while draft or rejected (exactly one of the two).
type UpdateInput struct {
	Company *CompanyData
	Person  *PersonData
}

// DocumentUpload is one uploaded file; the service sniffs the real type (A13).
type DocumentUpload struct {
	Kind              string
	RepresentativeDNI *string
	FileName          string
	ContentType       string
	Content           io.Reader
}

// Document is a stored document (the storage key never leaves the API).
type Document struct {
	ID                uuid.UUID
	Kind              string
	RepresentativeDNI *string
	FileName          string
	ContentType       string
	SizeBytes         int64
	CreatedAt         time.Time
}

// Decision is the ops verdict on a submitted case.
type Decision struct {
	Approved bool
	Reason   *string
}
