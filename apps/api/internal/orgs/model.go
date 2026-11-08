package orgs

import (
	"time"

	"github.com/google/uuid"

	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/money"
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
	// DocNumber is the KYC-verified document (CUIT, CUIL or DNI digits); nil before approval.
	// Filled by Get only.
	DocNumber *string
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
	FullName *string
	Role     auth.Role
	JoinedAt time.Time
}

// ApprovalMode says when a company payout needs a second person (owner's decision 2026-11-07).
type ApprovalMode string

// Approval modes (DB CHECK approval_policies.mode).
const (
	ApprovalAlways    ApprovalMode = "always"
	ApprovalThreshold ApprovalMode = "threshold"
)

// ApprovalPolicy is an org's payout approval policy. The zero rows of an org that never set one
// read as DefaultApprovalPolicy.
type ApprovalPolicy struct {
	Mode ApprovalMode
	// Thresholds in minor units per currency; a currency without one always needs approval.
	Thresholds             map[money.Currency]int64
	AllowOwnerSelfApproval bool
	UpdatedBy              *uuid.UUID
	UpdatedAt              *time.Time
}

// DefaultApprovalPolicy is 4-eyes always, owner self-approval allowed (effective only while the
// org has no other approver).
func DefaultApprovalPolicy() ApprovalPolicy {
	return ApprovalPolicy{Mode: ApprovalAlways, Thresholds: map[money.Currency]int64{}, AllowOwnerSelfApproval: true}
}

// ApprovalRules are what a payout decision needs: the policy and how many members besides the
// owner can approve. They hold THE approval rules (G12): payouts asks them, never re-derives.
type ApprovalRules struct {
	Policy                ApprovalPolicy
	ApproversBesidesOwner int64
}

// AutoApproves reports a company payout that needs no approval: threshold mode, a threshold for
// its currency, and an amount (fee not included) STRICTLY below it.
func (r ApprovalRules) AutoApproves(amount money.Amount) bool {
	limit, ok := r.Policy.Thresholds[amount.Currency]
	return r.Policy.Mode == ApprovalThreshold && ok && amount.Minor < limit
}

// OwnerMaySelfApprove reports whether the owner may approve their own payout: the policy allows
// it and nobody else in the org can approve.
func (r ApprovalRules) OwnerMaySelfApprove() bool {
	return r.Policy.AllowOwnerSelfApproval && r.ApproversBesidesOwner == 0
}

// PolicyInput replaces an org's approval policy.
type PolicyInput struct {
	Mode                   ApprovalMode
	Thresholds             map[money.Currency]int64
	AllowOwnerSelfApproval bool
}

// ERPInvitee is an ERP user the import invites, with the role mapped from their ERP permissions.
type ERPInvitee struct {
	Email string
	Role  auth.Role
}

// ERPInviteInput invites the ERP users of an imported org on behalf of the admin who linked it.
type ERPInviteInput struct {
	OrgID     uuid.UUID
	InvitedBy uuid.UUID
	Invitees  []ERPInvitee
}

// ERPInviteResult is what the import reports: who is invited (role after the cap) and how many
// were already members.
type ERPInviteResult struct {
	Invited        []ERPInvitee
	AlreadyMembers int
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
