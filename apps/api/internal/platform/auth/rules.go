package auth

type ruleKind int

const (
	kindDeny ruleKind = iota // the zero Rule denies: an unmarked operation is closed
	kindPublic
	kindSigned
	kindAuthenticated
	kindOrg
	kindInternal
)

// Rule is the ONE access marker of an operation (A14, AG15).
type Rule struct {
	kind    ruleKind
	minRole Role
	stepUp  bool
	kyc     bool
}

// Public needs no credentials (signup, login, JWKS, health). Each one is justified in review.
func Public() Rule { return Rule{kind: kindPublic} }

// Signed needs no bearer token; the handler verifies the provider signature over the raw
// body before reading it (AG16).
func Signed() Rule { return Rule{kind: kindSigned} }

// Authenticated needs a valid access token on an active session.
func Authenticated() Rule { return Rule{kind: kindAuthenticated} }

// OrgRole marks a route under /orgs/{orgId}: the caller must be a member with at least min.
// Not a member answers like a missing org (404, T2).
func OrgRole(min Role) Rule { return Rule{kind: kindOrg, minRole: min} }

// Internal needs the internal API key (ops endpoints, no UI in the MVP).
func Internal() Rule { return Rule{kind: kindInternal} }

// WithStepUp also requires a fresh step-up token bound to the session (R-ID-3): money
// movements, approvals, CVU creation.
func (r Rule) WithStepUp() Rule { r.stepUp = true; return r }

// WithKYC also requires the org's KYC to be approved (R-KYC-3).
func (r Rule) WithKYC() Rule { r.kyc = true; return r }

// IsDeny reports the zero rule.
func (r Rule) IsDeny() bool { return r.kind == kindDeny }

// NeedsToken reports whether the rule needs a bearer token. The contract declares
// `security: []` exactly on the operations whose rule needs none (TestAccessRulesCoverTheSpec).
func (r Rule) NeedsToken() bool { return r.kind == kindAuthenticated || r.kind == kindOrg }

// IsInternal reports the internal API key rule.
func (r Rule) IsInternal() bool { return r.kind == kindInternal }

// IsOrg reports a rule that resolves the org of the path.
func (r Rule) IsOrg() bool { return r.kind == kindOrg }

// MinRole is the minimum org role of an org rule.
func (r Rule) MinRole() Role { return r.minRole }

// NeedsStepUp reports the step-up requirement.
func (r Rule) NeedsStepUp() bool { return r.stepUp }

// NeedsKYC reports the approved-KYC requirement.
func (r Rule) NeedsKYC() bool { return r.kyc }

// Rules maps every operationId of the contract to its rule.
type Rules map[string]Rule
