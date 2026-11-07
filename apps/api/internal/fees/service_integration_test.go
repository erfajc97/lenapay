//go:build integration

package fees_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/fees"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db/dbtest"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
)

var pg *dbtest.Postgres

func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }

func setup(t *testing.T) (*pgxpool.Pool, fees.Service) {
	t.Helper()
	pool := pg.NewDatabase(t)
	svc := fees.NewService(fees.Deps{Pool: pool})
	if err := svc.EnsureDefaultPlan(t.Context()); err != nil {
		t.Fatal(err)
	}
	return pool, svc
}

func newOrg(t *testing.T, pool *pgxpool.Pool, planID *uuid.UUID) uuid.UUID {
	t.Helper()
	user, org := ids.New(), ids.New()
	if _, err := pool.Exec(t.Context(), `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, 'x')`, user, user.String()+"@t.local"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO organizations (id, kind, legal_name, created_by, fee_plan_id)
		VALUES ($1, 'company', 'Fees SA', $2, $3)`, org, user, planID); err != nil {
		t.Fatal(err)
	}
	return org
}

func TestEnsureDefaultPlanIsIdempotentAndKeepsOperatorChanges(t *testing.T) {
	t.Parallel()
	pool, svc := setup(t)
	if _, err := pool.Exec(t.Context(), `UPDATE fee_plan_rates SET payin_bps = 75 WHERE currency = 'ARS'`); err != nil {
		t.Fatal(err)
	}
	if err := svc.EnsureDefaultPlan(t.Context()); err != nil {
		t.Fatal(err)
	}
	var plans, rates, arsBps int
	if err := pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM fee_plans), (SELECT count(*) FROM fee_plan_rates),
		(SELECT payin_bps FROM fee_plan_rates WHERE currency = 'ARS')`).Scan(&plans, &rates, &arsBps); err != nil {
		t.Fatal(err)
	}
	if plans != 1 || rates != 2 || arsBps != 75 {
		t.Fatalf("plans = %d, rates = %d, ARS payin bps = %d; want 1, 2, 75 (operator change kept)", plans, rates, arsBps)
	}
}

func TestQuoteUsesTheOrgPlanOrTheDefault(t *testing.T) {
	t.Parallel()
	pool, svc := setup(t)
	custom := ids.New()
	if _, err := pool.Exec(t.Context(), `INSERT INTO fee_plans (id, name) VALUES ($1, 'vip')`, custom); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO fee_plan_rates (fee_plan_id, currency, payin_bps, payout_fixed_minor, payout_bps)
		VALUES ($1, 'ARS', 10, 777, 30)`, custom); err != nil {
		t.Fatal(err)
	}
	defaultOrg, vipOrg := newOrg(t, pool, nil), newOrg(t, pool, &custom)
	def := fees.DefaultRates()[money.ARS]

	q, err := svc.Quote(t.Context(), defaultOrg, fees.OperationPayout, money.New(100_000, money.ARS))
	if err != nil {
		t.Fatal(err)
	}
	wantDefault, err := fees.Compute(fees.OperationPayout, money.New(100_000, money.ARS), def)
	if err != nil {
		t.Fatal(err)
	}
	if q.Fee != wantDefault || q.Bps != def.PayoutBps || q.FixedMinor != def.PayoutFixedMinor || q.PlanID == custom {
		t.Fatalf("default org quote = %+v, want fee %v from the default plan", q, wantDefault)
	}

	q, err = svc.Quote(t.Context(), vipOrg, fees.OperationPayout, money.New(100_000, money.ARS))
	if err != nil {
		t.Fatal(err)
	}
	if q.Fee != money.New(777+300, money.ARS) || q.PlanID != custom || q.Bps != 30 || q.FixedMinor != 777 {
		t.Fatalf("vip payout quote = %+v, want 1077 ARS from plan %s", q, custom)
	}
	q, err = svc.Quote(t.Context(), vipOrg, fees.OperationPayin, money.New(100_000, money.ARS))
	if err != nil {
		t.Fatal(err)
	}
	if q.Fee != money.New(100, money.ARS) || q.Bps != 10 || q.FixedMinor != 0 {
		t.Fatalf("vip payin quote = %+v, want 100 ARS at 10 bps", q)
	}
	// The vip plan has no USD rate: a configuration error, never a zero fee.
	if _, err := svc.Quote(t.Context(), vipOrg, fees.OperationPayin, money.New(100, money.USD)); err == nil {
		t.Fatal("quote without a rate for the currency: want an error, got a fee")
	}
}

func TestQuoteRejectsBadAmounts(t *testing.T) {
	t.Parallel()
	pool, svc := setup(t)
	org := newOrg(t, pool, nil)
	for name, tc := range map[string]struct {
		amount money.Amount
		code   apperr.Code
	}{
		"zero":        {money.New(0, money.ARS), apperr.CodeInvalidAmount},
		"negative":    {money.New(-1, money.ARS), apperr.CodeInvalidAmount},
		"unsupported": {money.New(100, "EUR"), apperr.CodeCurrencyMismatch},
	} {
		if _, err := svc.Quote(t.Context(), org, fees.OperationPayin, tc.amount); !apperr.IsCode(err, tc.code) {
			t.Fatalf("%s: err = %v, want %s", name, err, tc.code)
		}
	}
	if _, err := svc.Quote(t.Context(), org, fees.OperationPayin, money.New(1, money.ARS)); err != nil {
		t.Fatalf("smallest positive amount: %v", err)
	}
}
