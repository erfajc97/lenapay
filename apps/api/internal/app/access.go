package app

import "github.com/erfajc97/lenapay/apps/api/internal/platform/auth"

// AccessRules is THE access table: EXACTLY one rule per operationId of the contract (A14,
// AG15). An operation missing here is denied at runtime and fails
// TestAccessRulesCoverTheSpec. Lanes edit only their own rows. A function, not a variable,
// so nothing can mutate it at runtime.
func AccessRules() auth.Rules {
	viewer, operator, approver, admin := auth.RoleViewer, auth.RoleOperator, auth.RoleApprover, auth.RoleAdmin
	return auth.Rules{
		// system
		"getHealth": auth.Public(),

		// identity
		"signup":                auth.Public(),
		"verifyEmail":           auth.Public(),
		"login":                 auth.Public(),
		"refreshToken":          auth.Public(),
		"getJwks":               auth.Public(),
		"logout":                auth.Authenticated(),
		"createStepUpChallenge": auth.Authenticated(),
		"verifyStepUp":          auth.Authenticated(),
		"registerDevice":        auth.Authenticated(),
		"getMe":                 auth.Authenticated(),

		// orgs
		"listOrgs":     auth.Authenticated(),
		"createOrg":    auth.Authenticated(),
		"acceptInvite": auth.Authenticated(),
		"getOrg":       auth.OrgRole(viewer),
		"listMembers":  auth.OrgRole(viewer),
		"addMember":    auth.OrgRole(admin),
		"createInvite": auth.OrgRole(admin),

		// kyc
		"getKyc":            auth.OrgRole(viewer),
		"updateKyc":         auth.OrgRole(admin),
		"uploadKycDocument": auth.OrgRole(admin),
		"submitKyc":         auth.OrgRole(admin),
		"decideKyc":         auth.Internal(),

		// accounts
		"listAccounts":           auth.OrgRole(viewer),
		"getBalances":            auth.OrgRole(viewer),
		"createAccount":          auth.OrgRole(admin).WithKYC(),
		"updateAccount":          auth.OrgRole(admin).WithKYC(),
		"createInternalTransfer": auth.OrgRole(operator).WithKYC().WithStepUp(),

		// payers / CVU
		"listPayers":  auth.OrgRole(viewer),
		"getPayer":    auth.OrgRole(viewer),
		"createPayer": auth.OrgRole(operator).WithKYC(),
		"updatePayer": auth.OrgRole(operator).WithKYC(),
		"createCvu":   auth.OrgRole(operator).WithKYC().WithStepUp(),
		"updateCvu":   auth.OrgRole(operator).WithKYC(),
		"disableCvu":  auth.OrgRole(operator).WithKYC(),

		// payins
		"receivePspTransferWebhook": auth.Signed(),
		"getPayin":                  auth.OrgRole(viewer),

		// payouts
		"resolveDestination": auth.OrgRole(operator).WithKYC(),
		"quotePayout":        auth.OrgRole(operator).WithKYC(),
		"createPayout":       auth.OrgRole(operator).WithKYC().WithStepUp(),
		"getPayout":          auth.OrgRole(viewer),
		"getPayoutProof":     auth.OrgRole(viewer),
		"cancelPayout":       auth.OrgRole(operator).WithKYC(),
		"listApprovals":      auth.OrgRole(approver),
		"approvePayout":      auth.OrgRole(approver).WithKYC().WithStepUp(),
		"rejectPayout":       auth.OrgRole(approver).WithKYC().WithStepUp(),

		// movements
		"listMovements": auth.OrgRole(viewer),

		// reconciliation
		"listReconciliationRuns": auth.OrgRole(admin),
		"runReconciliation":      auth.Internal(),

		// erp link
		"getErpLink":          auth.OrgRole(viewer),
		"getErpLinkCandidate": auth.OrgRole(admin).WithKYC(),
		"createErpLink":       auth.OrgRole(admin).WithKYC(),

		// realtime / notifications
		"streamOrgEvents":   auth.OrgRole(viewer),
		"registerPushToken": auth.Authenticated(),
	}
}
