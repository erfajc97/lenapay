package erplink

import (
	"testing"

	"lenapay/apps/api/internal/platform/auth"
)

// The mapping decides who can move money in Lenapay after an ERP link: a wrong row gives a
// read-only ERP user an operator's power, or an admin's to a payouts clerk.
func TestRoleForERPUser(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		user User
		want auth.Role
		ok   bool
	}{
		{"admin flag", User{Admin: true}, auth.RoleAdmin, true},
		{"org configuration is the ERP admin", User{Permissions: []string{"CONFIGURACION_ORGANIZACION"}}, auth.RoleAdmin, true},
		{"reconciliations master", User{Permissions: []string{"PAYMENTS_RECONCILIATIONS_MASTER"}}, auth.RoleAdmin, true},
		{"payouts", User{Permissions: []string{"PAYMENTS_PAYOUTS", "CONTABILIDAD"}}, auth.RoleOperator, true},
		{"create accounts only", User{Permissions: []string{"PAYMENTS_CREATE_ACCOUNTS"}}, auth.RoleOperator, true},
		{"create cvu only", User{Permissions: []string{"PAYMENTS_CREATE_CVU"}}, auth.RoleOperator, true},
		{"movements between accounts", User{Permissions: []string{"PAYMENTS_ACCOUNT_MOVEMENTS"}}, auth.RoleOperator, true},
		{"builder twin of payouts", User{Permissions: []string{"CONSTRUCTORA_PAYMENTS_PAYOUTS"}}, auth.RoleOperator, true},
		{"reconciliations read-only", User{Permissions: []string{"PAYMENTS_RECONCILIATIONS", "TESORERIA"}}, auth.RoleViewer, true},
		{"document approvals are not payment approvals", User{Permissions: []string{"APROBACIONES", "GENERAL_APROBACIONES"}}, "", false},
		{"no payments access", User{Permissions: []string{"CONTABILIDAD", "CLIENTES"}}, "", false},
		{"builder non-payments code stays foreign", User{Permissions: []string{"CONSTRUCTORA_COMPRAS"}}, "", false},
		{"no permissions", User{}, "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := RoleForERPUser(tt.user)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("RoleForERPUser = %q, %v; want %q, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}
