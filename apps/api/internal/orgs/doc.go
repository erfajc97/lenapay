// Package orgs owns organizations, memberships and invites (lane A, R-ORG-1/2). The role
// order of auth.Roles is the ONE permission matrix (T4): nobody grants above their own role,
// nobody changes their own role, and the last admin cannot be removed. It also answers the
// cross-feature questions about an org: its memberships (identity /me), its KYC status
// (written by kyc) and its CUIT (erplink).
package orgs
