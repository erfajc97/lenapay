//go:build integration

package auth_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db/dbtest"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
)

var pg *dbtest.Postgres

func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }

func TestPGDirectory(t *testing.T) {
	t.Parallel()
	pool := pg.NewDatabase(t)
	ctx := t.Context()
	user, other, session, revoked, org := ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO users (id, email, password_hash) VALUES ($1, 'a@lenapay.test', 'x'), ($2, 'b@lenapay.test', 'x')", []any{user, other}},
		{"INSERT INTO sessions (id, user_id) VALUES ($1, $2)", []any{session, user}},
		{"INSERT INTO sessions (id, user_id, revoked_at) VALUES ($1, $2, now())", []any{revoked, user}},
		{"INSERT INTO organizations (id, kind, legal_name, kyc_status, created_by) VALUES ($1, 'company', 'Acme SA', 'approved', $2)", []any{org, user}},
		{"INSERT INTO memberships (org_id, user_id, role) VALUES ($1, $2, 'approver')", []any{org, user}},
	} {
		if _, err := pool.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatal(err)
		}
	}
	dir := auth.NewPGDirectory(pool)

	if err := dir.ActiveSession(ctx, user, session); err != nil {
		t.Fatalf("active session: %v", err)
	}
	if err := dir.ActiveSession(ctx, user, revoked); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("revoked session err = %v", err)
	}
	if err := dir.ActiveSession(ctx, other, session); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("session of another user err = %v", err)
	}
	m, err := dir.Membership(ctx, org, user)
	if err != nil || m.Role != auth.RoleApprover || m.KYCStatus != auth.KYCApproved {
		t.Fatalf("membership = %+v, %v", m, err)
	}
	if _, err := dir.Membership(ctx, org, other); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("non-member err = %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE users SET status = 'disabled' WHERE id = $1", user); err != nil {
		t.Fatal(err)
	}
	if err := dir.ActiveSession(ctx, user, session); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("disabled user err = %v", err)
	}
}
