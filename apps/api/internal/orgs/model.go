package orgs

import (
	"time"

	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
)

// Kind is the kind of organization.
type Kind string

// Organization kinds (DB CHECK organizations.kind).
const (
	KindCompany  Kind = "company"
	KindPersonal Kind = "personal"
)

// Organization is an org as seen by one member.
type Organization struct {
	ID        uuid.UUID
	Kind      Kind
	LegalName string
	CUIT      *string
	KYCStatus string
	// Role is the caller's role in this org.
	Role      auth.Role
	CreatedAt time.Time
}

// CreateInput creates an org; the caller becomes its admin. personal needs PERSONAL_MODE_ENABLED.
type CreateInput struct {
	Kind      Kind
	LegalName string
	CUIT      *string
}

// Member is a user of an org.
type Member struct {
	UserID   uuid.UUID
	Email    string
	Role     auth.Role
	JoinedAt time.Time
}

// Document types of a verified identity (DB CHECK organizations.doc_type).
const (
	DocTypeCUIT = "cuit"
	DocTypeDNI  = "dni"
)

// VerifiedIdentity is what an approved KYC case proved: a company's legal name and CUIT, or a
// person's full name and DNI.
type VerifiedIdentity struct {
	LegalName string
	DocType   string
	DocNumber string
}

// MemberInput adds a member or invites one.
type MemberInput struct {
	Email string
	Role  auth.Role
}

// Invite is a pending invitation (the token only travels by email).
type Invite struct {
	ID        uuid.UUID
	Email     string
	Role      auth.Role
	ExpiresAt time.Time
	CreatedAt time.Time
}
