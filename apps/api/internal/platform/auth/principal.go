// Package auth is the access layer: EdDSA tokens of the local identity provider (R-ID-4), the
// JWKS document, step-up tokens (R-ID-3), and the guard that enforces the route table with
// DEFAULT DENY (A14, AG15) before any handler, validator or hand-mounted route runs.
package auth

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
)

// Role is a member's role inside an organization.
type Role string

// Roles from least to most privileged (R-ORG, PRD §2).
const (
	RoleViewer   Role = "viewer"
	RoleOperator Role = "operator"
	RoleApprover Role = "approver"
	RoleAdmin    Role = "admin"
)

// Roles returns every role in privilege order. This ordering is the ONE permission matrix (T4).
func Roles() []Role {
	return []Role{RoleViewer, RoleOperator, RoleApprover, RoleAdmin}
}

// ParseRole validates a role.
func ParseRole(s string) (Role, bool) {
	r := Role(s)
	return r, slices.Contains(Roles(), r)
}

// AtLeast reports whether r grants everything min grants. Unknown roles grant nothing.
func (r Role) AtLeast(min Role) bool {
	have, want := slices.Index(Roles(), r), slices.Index(Roles(), min)
	return have >= 0 && want >= 0 && have >= want
}

// KYC statuses of an organization.
const (
	KYCDraft     = "draft"
	KYCSubmitted = "submitted"
	KYCApproved  = "approved"
	KYCRejected  = "rejected"
)

// Principal is the authenticated caller of a request (T3: the tenant comes from the session,
// checked against membership — never from a body or query).
type Principal struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	// OrgID, Role and KYCStatus are set only on routes under /orgs/{orgId}.
	OrgID     uuid.UUID
	Role      Role
	KYCStatus string
	// StepUp is true when the request carried a valid step-up token.
	StepUp bool
}

// HasOrg reports whether the principal was resolved for an organization route.
func (p Principal) HasOrg() bool { return p.OrgID != uuid.Nil }

type principalKey struct{}

// WithPrincipal stores p in ctx (the guard does it; tests of handlers may too).
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom returns the principal of the request.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// RequirePrincipal returns the principal or auth.unauthenticated. Handlers of authenticated
// routes call it; the guard guarantees it is present, so a miss is a wiring bug that fails closed.
func RequirePrincipal(ctx context.Context) (Principal, error) {
	p, ok := PrincipalFrom(ctx)
	if !ok {
		return Principal{}, apperr.New(apperr.CodeUnauthenticated, nil)
	}
	return p, nil
}
