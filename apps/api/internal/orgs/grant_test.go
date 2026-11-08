package orgs

import (
	"testing"

	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/auth"
)

// T4: nobody grants a role above their own; equal is allowed (the last passing value).
func TestCheckGrant(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		caller auth.Role
		grant  auth.Role
		want   apperr.Code
	}{
		{"admin grants admin", auth.RoleAdmin, auth.RoleAdmin, ""},
		{"admin grants viewer", auth.RoleAdmin, auth.RoleViewer, ""},
		{"approver grants approver", auth.RoleApprover, auth.RoleApprover, ""},
		{"approver cannot grant admin", auth.RoleApprover, auth.RoleAdmin, apperr.CodeRoleEscalation},
		{"operator cannot grant approver", auth.RoleOperator, auth.RoleApprover, apperr.CodeRoleEscalation},
		{"unknown role", auth.RoleAdmin, auth.Role("owner"), apperr.CodeValidation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := checkGrant(auth.Principal{Role: tt.caller}, tt.grant)
			if apperr.CodeOf(err) != tt.want {
				t.Fatalf("checkGrant = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestInviteToken(t *testing.T) {
	t.Parallel()
	tok := newInviteToken()
	if len(tok) != 43 || tok == newInviteToken() {
		t.Fatalf("token %q: want 43 random base64url chars (contract: 16..128)", tok)
	}
	if string(inviteTokenHash(tok)) == string(inviteTokenHash(tok+"x")) {
		t.Fatal("hashes of different tokens collide")
	}
}
