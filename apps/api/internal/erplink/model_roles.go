package erplink

import (
	"slices"
	"strings"

	"lenapay/apps/api/internal/platform/auth"
)

// Lebane ERP permission codes (lebane-back PermisoUsuario) that matter to Lenapay. Payments
// permissions exist per vertical: the developer ("Desarrolladora") codes and their builder
// ("Constructora") twins (PermisosPaymentsPorVertical in lebane-back).
const (
	permPayouts               = "PAYMENTS_PAYOUTS"
	permCreateAccounts        = "PAYMENTS_CREATE_ACCOUNTS"
	permCreateCVU             = "PAYMENTS_CREATE_CVU"
	permAccountMovements      = "PAYMENTS_ACCOUNT_MOVEMENTS"
	permReconciliations       = "PAYMENTS_RECONCILIATIONS"
	permReconciliationsMaster = "PAYMENTS_RECONCILIATIONS_MASTER"
	// permOrgConfiguration is Lebane's organization admin ("full access to the configuration:
	// users, …"); the fixtures' admin flag stands for the same thing.
	permOrgConfiguration = "CONFIGURACION_ORGANIZACION"
	builderPrefix        = "CONSTRUCTORA_"
	paymentsPrefix       = "PAYMENTS_"
)

// RoleForERPUser maps an ERP user's permissions to the Lenapay role they are invited with
// (docs/architecture.md "ERP users import"). ok is false for a user with no payments access at
// all: they are not invited. Lebane has no payments approval permission (its APROBACIONES codes
// approve purchase documents, not payments), so nobody maps to approver: operators are promoted
// by the owner or an admin in Lenapay.
func RoleForERPUser(u User) (role auth.Role, ok bool) {
	perms := normalizePermissions(u.Permissions)
	has := func(code string) bool { return slices.Contains(perms, code) }
	switch {
	case u.Admin || has(permOrgConfiguration) || has(permReconciliationsMaster):
		return auth.RoleAdmin, true
	case has(permPayouts) || has(permCreateAccounts) || has(permCreateCVU) || has(permAccountMovements):
		return auth.RoleOperator, true
	case has(permReconciliations):
		return auth.RoleViewer, true
	default:
		return "", false
	}
}

// normalizePermissions folds the builder twins onto the developer codes
// (CONSTRUCTORA_PAYMENTS_PAYOUTS → PAYMENTS_PAYOUTS): both grant the same in Lenapay.
func normalizePermissions(perms []string) []string {
	out := make([]string, 0, len(perms))
	for _, p := range perms {
		if twin, found := strings.CutPrefix(p, builderPrefix); found && strings.HasPrefix(twin, paymentsPrefix) {
			p = twin
		}
		out = append(out, p)
	}
	return out
}
