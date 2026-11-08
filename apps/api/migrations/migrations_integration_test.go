//go:build integration

package migrations_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lenapay/apps/api/internal/platform/db"
	"lenapay/apps/api/internal/platform/db/dbtest"
	"lenapay/apps/api/internal/platform/ids"
)

var pg *dbtest.Postgres

func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }

// AG10 drift test: every migration applies to an empty database, rolls all the way back,
// and applies again.
func TestMigrationsUpDownUp(t *testing.T) {
	t.Parallel()
	dsn := pg.EmptyDatabase(t)
	for step, run := range []func(context.Context, string) error{db.Migrate, db.MigrateDownToZero, db.Migrate} {
		if err := run(t.Context(), dsn); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
	}
}

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// ledgerFixture creates two ARS ledger accounts: a system one that may go negative and an
// org one that may not.
func ledgerFixture(t *testing.T, pool *pgxpool.Pool) (system, org uuid.UUID) {
	t.Helper()
	user, orgID := ids.New(), ids.New()
	sysAcc, orgAcc := ids.New(), ids.New()
	system, org = ids.New(), ids.New()
	mustExec(t, pool, "INSERT INTO users (id, email, password_hash) VALUES ($1, 'u@lenapay.test', 'x')", user)
	mustExec(t, pool, "INSERT INTO organizations (id, kind, legal_name, created_by) VALUES ($1, 'company', 'Acme SA', $2)", orgID, user)
	mustExec(t, pool, "INSERT INTO accounts (id, org_id, kind, system_code, name) VALUES ($1, NULL, 'system', 'psp_master', 'PSP master')", sysAcc)
	mustExec(t, pool, "INSERT INTO accounts (id, org_id, kind, name) VALUES ($1, $2, 'main', 'Main')", orgAcc, orgID)
	mustExec(t, pool, "INSERT INTO ledger_accounts (id, account_id, currency) VALUES ($1, $2, 'ARS'), ($3, $4, 'ARS')",
		system, sysAcc, org, orgAcc)
	mustExec(t, pool, "INSERT INTO ledger_balances (ledger_account_id, allow_negative) VALUES ($1, true), ($2, false)", system, org)
	return system, org
}

type leg struct {
	account  uuid.UUID
	currency string
	minor    int64
}

func post(ctx context.Context, pool *pgxpool.Pool, legs ...leg) error {
	return db.WithTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		txID := ids.New()
		if _, err := tx.Exec(ctx, `INSERT INTO ledger_transactions (id, kind, ref_type, ref_id) VALUES ($1, 'adjustment', 'test', $2)`,
			txID, ids.New()); err != nil {
			return err
		}
		for _, l := range legs {
			if _, err := tx.Exec(ctx, `INSERT INTO ledger_postings (id, transaction_id, ledger_account_id, currency, amount_minor)
				VALUES ($1, $2, $3, $4, $5)`, ids.New(), txID, l.account, l.currency, l.minor); err != nil {
				return err
			}
		}
		return nil
	})
}

// R-ACC-2: every ledger transaction sums to zero; the database rejects it at COMMIT otherwise.
func TestLedgerTransactionsMustBalance(t *testing.T) {
	t.Parallel()
	pool := pg.NewDatabase(t)
	ctx := t.Context()
	system, org := ledgerFixture(t, pool)

	if err := post(ctx, pool, leg{system, "ARS", -1000}, leg{org, "ARS", 1000}); err != nil {
		t.Fatalf("balanced transaction rejected: %v", err)
	}
	// Off by one cent: the deferred trigger fires at commit.
	if err := post(ctx, pool, leg{system, "ARS", -1000}, leg{org, "ARS", 999}); err == nil || !strings.Contains(err.Error(), "23514") {
		t.Fatalf("unbalanced transaction err = %v, want check_violation", err)
	}
	if err := post(ctx, pool, leg{org, "ARS", 1}); err == nil {
		t.Fatal("single-leg transaction accepted")
	}
}

func TestPostingCurrencyMustMatchItsAccount(t *testing.T) {
	t.Parallel()
	pool := pg.NewDatabase(t)
	system, org := ledgerFixture(t, pool)
	if err := post(t.Context(), pool, leg{system, "USD", -5}, leg{org, "USD", 5}); err == nil {
		t.Fatal("USD postings on ARS ledger accounts accepted")
	}
}

func TestLedgerIsAppendOnly(t *testing.T) {
	t.Parallel()
	pool := pg.NewDatabase(t)
	ctx := t.Context()
	system, org := ledgerFixture(t, pool)
	if err := post(ctx, pool, leg{system, "ARS", -10}, leg{org, "ARS", 10}); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		"UPDATE ledger_postings SET amount_minor = amount_minor * 2",
		"DELETE FROM ledger_postings",
		"UPDATE ledger_transactions SET memo = 'x'",
		"DELETE FROM ledger_transactions",
	} {
		if _, err := pool.Exec(ctx, stmt); err == nil {
			t.Errorf("%q succeeded on an append-only table", stmt)
		}
	}
}

// Available = posted − held never goes below zero on a non-system account (R-ACC-2).
func TestBalanceCannotGoBelowHolds(t *testing.T) {
	t.Parallel()
	pool := pg.NewDatabase(t)
	system, org := ledgerFixture(t, pool)
	mustExec(t, pool, "UPDATE ledger_balances SET posted_minor = 100 WHERE ledger_account_id = $1", org)
	// Boundary pair (FT1): holding exactly the balance is fine, one more cent is not.
	mustExec(t, pool, "UPDATE ledger_balances SET held_minor = 100 WHERE ledger_account_id = $1", org)
	if _, err := pool.Exec(t.Context(), "UPDATE ledger_balances SET held_minor = 101 WHERE ledger_account_id = $1", org); err == nil {
		t.Fatal("hold above the balance accepted")
	}
	mustExec(t, pool, "UPDATE ledger_balances SET posted_minor = -5 WHERE ledger_account_id = $1", system)
}

// R-OUT-4 backstop: a payout's approver can never be its creator, even if code forgets.
func TestPayoutCreatorCannotApprove(t *testing.T) {
	t.Parallel()
	pool := pg.NewDatabase(t)
	user, orgID, acc, payout := ids.New(), ids.New(), ids.New(), ids.New()
	mustExec(t, pool, "INSERT INTO users (id, email, password_hash) VALUES ($1, 'u@lenapay.test', 'x')", user)
	mustExec(t, pool, "INSERT INTO organizations (id, kind, legal_name, created_by) VALUES ($1, 'company', 'Acme SA', $2)", orgID, user)
	mustExec(t, pool, "INSERT INTO accounts (id, org_id, kind, name) VALUES ($1, $2, 'main', 'Main')", acc, orgID)
	mustExec(t, pool, `INSERT INTO payouts (id, org_id, account_id, currency, amount_minor, fee_minor, destination_type,
			destination_value, destination_account, destination_holder_name, destination_holder_cuit,
			concept, status, created_by, idempotency_key)
		VALUES ($1, $2, $3, 'ARS', 1000, 10, 'cbu', '0000003100000000000001', '0000003100000000000001',
			'Juan Perez', '20123456789', 'Rent', 'pending_approval', $4, 'client-key-0000000001')`,
		payout, orgID, acc, user)
	if _, err := pool.Exec(t.Context(), "UPDATE payouts SET status = 'approved', approved_by = created_by WHERE id = $1", payout); err == nil {
		t.Fatal("self-approval accepted by the database")
	}
}
