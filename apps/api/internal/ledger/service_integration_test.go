//go:build integration

package ledger_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"lenapay/apps/api/internal/ledger"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/clock"
	"lenapay/apps/api/internal/platform/db"
	"lenapay/apps/api/internal/platform/db/dbtest"
	"lenapay/apps/api/internal/platform/ids"
	"lenapay/apps/api/internal/platform/money"
)

var pg *dbtest.Postgres

func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }

type env struct {
	pool   *pgxpool.Pool
	svc    ledger.Service
	org    uuid.UUID
	master uuid.UUID
	fees   uuid.UUID
}

func setup(t *testing.T) env {
	t.Helper()
	ctx := t.Context()
	pool := pg.NewDatabase(t)
	svc := ledger.NewService(ledger.Deps{Pool: pool, Clock: clock.NewFake(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))})
	if err := svc.EnsureSystemAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	user, org := ids.New(), ids.New()
	mustExec(t, pool, `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, 'x')`, user, user.String()+"@test.local")
	mustExec(t, pool, `INSERT INTO organizations (id, kind, legal_name, kyc_status, created_by)
		VALUES ($1, 'company', 'Ledger Test SA', 'approved', $2)`, org, user)
	master, err := svc.SystemAccountID(ctx, ledger.SystemPSPMaster)
	if err != nil {
		t.Fatal(err)
	}
	fees, err := svc.SystemAccountID(ctx, ledger.SystemFeesRevenue)
	if err != nil {
		t.Fatal(err)
	}
	return env{pool: pool, svc: svc, org: org, master: master, fees: fees}
}

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// newAccount opens a customer (project) account with its ledger accounts.
func (e env) newAccount(t *testing.T) uuid.UUID {
	t.Helper()
	id := ids.New()
	mustExec(t, e.pool, `INSERT INTO accounts (id, org_id, kind, name) VALUES ($1, $2, 'project', 'P')`, id, e.org)
	if err := db.WithTx(t.Context(), e.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return e.svc.OpenAccount(t.Context(), tx, id)
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

func (e env) post(ctx context.Context, entry ledger.Entry) (uuid.UUID, error) {
	var txID uuid.UUID
	err := db.WithTx(ctx, e.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var err error
		txID, err = e.svc.Post(ctx, tx, entry)
		return err
	})
	return txID, err
}

// fund credits account like a payin: the PSP master account goes negative, the customer up.
func (e env) fund(t *testing.T, account uuid.UUID, minor int64, c money.Currency) {
	t.Helper()
	if _, err := e.post(t.Context(), transfer(e.master, account, minor, c, ledger.KindPayin)); err != nil {
		t.Fatal(err)
	}
}

func transfer(from, to uuid.UUID, minor int64, c money.Currency, kind ledger.Kind) ledger.Entry {
	return ledger.Entry{
		Kind: kind, RefType: string(kind), RefID: ids.New(),
		Legs: []ledger.Leg{
			{AccountID: from, Amount: money.New(-minor, c)},
			{AccountID: to, Amount: money.New(minor, c)},
		},
	}
}

func (e env) balance(t *testing.T, account uuid.UUID, c money.Currency) ledger.Balance {
	t.Helper()
	all, err := e.svc.Balances(t.Context(), []uuid.UUID{account})
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range all {
		if b.Posted.Currency == c {
			return b
		}
	}
	t.Fatalf("no %s balance for %s", c, account)
	return ledger.Balance{}
}

func assertBalance(t *testing.T, b ledger.Balance, posted, held, available int64) {
	t.Helper()
	if b.Posted.Minor != posted || b.Held.Minor != held || b.Available.Minor != available {
		t.Fatalf("balance = posted %d held %d available %d, want %d %d %d",
			b.Posted.Minor, b.Held.Minor, b.Available.Minor, posted, held, available)
	}
}

// assertLedgerSumsToZero: the whole ledger nets to zero per currency, and every cached balance
// equals the sum of its postings (the cache never drifts from the journal).
func assertLedgerSumsToZero(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	var nonZero, drift int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM (SELECT currency FROM ledger_postings
		GROUP BY currency HAVING sum(amount_minor) <> 0) s`).Scan(&nonZero); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM ledger_balances b
		WHERE b.posted_minor <> COALESCE((SELECT sum(p.amount_minor) FROM ledger_postings p
		WHERE p.ledger_account_id = b.ledger_account_id), 0)`).Scan(&drift); err != nil {
		t.Fatal(err)
	}
	if nonZero != 0 || drift != 0 {
		t.Fatalf("ledger invariants broken: %d unbalanced currencies, %d drifted balances", nonZero, drift)
	}
}

func TestEnsureSystemAccountsIsIdempotent(t *testing.T) {
	t.Parallel()
	e := setup(t)
	if err := e.svc.EnsureSystemAccounts(t.Context()); err != nil {
		t.Fatal(err)
	}
	var accounts, ledgerAccounts, negativeAllowed int
	if err := e.pool.QueryRow(t.Context(), `SELECT count(*) FROM accounts WHERE kind = 'system'`).Scan(&accounts); err != nil {
		t.Fatal(err)
	}
	if err := e.pool.QueryRow(t.Context(), `SELECT count(*), count(*) FILTER (WHERE b.allow_negative)
		FROM ledger_accounts la JOIN accounts a ON a.id = la.account_id
		JOIN ledger_balances b ON b.ledger_account_id = la.id WHERE a.kind = 'system'`).Scan(&ledgerAccounts, &negativeAllowed); err != nil {
		t.Fatal(err)
	}
	if accounts != 4 || ledgerAccounts != 8 || negativeAllowed != 8 {
		t.Fatalf("system accounts = %d, ledger accounts = %d (allow negative %d); want 4, 8, 8", accounts, ledgerAccounts, negativeAllowed)
	}
	again, err := e.svc.SystemAccountID(t.Context(), ledger.SystemPSPMaster)
	if err != nil || again != e.master {
		t.Fatalf("psp_master id changed: %s → %s (%v)", e.master, again, err)
	}
}

func TestOpenAccountCreatesNonNegativeBalances(t *testing.T) {
	t.Parallel()
	e := setup(t)
	acc := e.newAccount(t)
	if err := db.WithTx(t.Context(), e.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return e.svc.OpenAccount(t.Context(), tx, acc) // idempotent
	}); err != nil {
		t.Fatal(err)
	}
	var n, negative int
	if err := e.pool.QueryRow(t.Context(), `SELECT count(*), count(*) FILTER (WHERE b.allow_negative)
		FROM ledger_accounts la JOIN ledger_balances b ON b.ledger_account_id = la.id
		WHERE la.account_id = $1`, acc).Scan(&n, &negative); err != nil {
		t.Fatal(err)
	}
	if n != 2 || negative != 0 {
		t.Fatalf("ledger accounts = %d (allow negative %d), want 2 (0)", n, negative)
	}
}

func TestPostUpdatesBalancesAndKeepsInvariants(t *testing.T) {
	t.Parallel()
	e := setup(t)
	acc := e.newAccount(t)
	e.fund(t, acc, 1000, money.ARS)
	e.fund(t, acc, 250, money.USD)

	assertBalance(t, e.balance(t, acc, money.ARS), 1000, 0, 1000)
	assertBalance(t, e.balance(t, acc, money.USD), 250, 0, 250)
	assertBalance(t, e.balance(t, e.master, money.ARS), -1000, 0, -1000) // system accounts may go negative
	assertLedgerSumsToZero(t, e.pool)
}

func TestPostWithFeeLegsSplitsTheGross(t *testing.T) {
	t.Parallel()
	e := setup(t)
	acc := e.newAccount(t)
	entry := ledger.Entry{
		Kind: ledger.KindPayin, RefType: "payin", RefID: ids.New(), OrgID: &e.org, Memo: "payin",
		Legs: []ledger.Leg{
			{AccountID: e.master, Amount: money.New(-10_000, money.ARS)},
			{AccountID: acc, Amount: money.New(9_950, money.ARS)},
			{AccountID: e.fees, Amount: money.New(50, money.ARS)},
		},
	}
	txID, err := e.post(t.Context(), entry)
	if err != nil {
		t.Fatal(err)
	}
	var legs int
	if err := e.pool.QueryRow(t.Context(), `SELECT count(*) FROM ledger_postings WHERE transaction_id = $1`, txID).Scan(&legs); err != nil {
		t.Fatal(err)
	}
	if legs != 3 {
		t.Fatalf("postings = %d, want 3", legs)
	}
	assertBalance(t, e.balance(t, acc, money.ARS), 9_950, 0, 9_950)
	assertBalance(t, e.balance(t, e.fees, money.ARS), 50, 0, 50)
	assertLedgerSumsToZero(t, e.pool)
}

func TestPostRejectsMalformedEntries(t *testing.T) {
	t.Parallel()
	e := setup(t)
	acc := e.newAccount(t)
	ghost := ids.New()
	leg := func(account uuid.UUID, minor int64, c money.Currency) ledger.Leg {
		return ledger.Leg{AccountID: account, Amount: money.New(minor, c)}
	}
	cases := []struct {
		name    string
		legs    []ledger.Leg
		kind    ledger.Kind
		wantErr error
		code    apperr.Code
	}{
		{"one leg", []ledger.Leg{leg(acc, 100, money.ARS)}, ledger.KindAdjustment, ledger.ErrInvalidEntry, ""},
		{"zero leg", []ledger.Leg{leg(e.master, 0, money.ARS), leg(acc, 0, money.ARS)}, ledger.KindAdjustment, ledger.ErrInvalidEntry, ""},
		{"unknown kind", []ledger.Leg{leg(e.master, -1, money.ARS), leg(acc, 1, money.ARS)}, ledger.Kind("gift"), ledger.ErrInvalidEntry, ""},
		{"unbalanced", []ledger.Leg{leg(e.master, -100, money.ARS), leg(acc, 99, money.ARS)}, ledger.KindAdjustment, ledger.ErrUnbalanced, ""},
		{"cross currency", []ledger.Leg{leg(e.master, -100, money.ARS), leg(acc, 100, money.USD)}, ledger.KindAdjustment, ledger.ErrUnbalanced, apperr.CodeCurrencyMismatch},
		{"unsupported currency", []ledger.Leg{leg(e.master, -100, "EUR"), leg(acc, 100, "EUR")}, ledger.KindAdjustment, nil, apperr.CodeCurrencyMismatch},
		{"unknown account", []ledger.Leg{leg(e.master, -100, money.ARS), leg(ghost, 100, money.ARS)}, ledger.KindAdjustment, nil, apperr.CodeAccountNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := e.post(t.Context(), ledger.Entry{Kind: tc.kind, RefType: "test", RefID: ids.New(), Legs: tc.legs})
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if tc.code != "" && apperr.CodeOf(err) != tc.code {
				t.Fatalf("code = %q, want %q (err %v)", apperr.CodeOf(err), tc.code, err)
			}
		})
	}
	var rows int
	if err := e.pool.QueryRow(t.Context(), `SELECT count(*) FROM ledger_transactions`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatalf("rejected entries wrote %d transactions", rows)
	}
}

func TestPostNeverOverdrawsACustomerAccount(t *testing.T) {
	t.Parallel()
	e := setup(t)
	from, to := e.newAccount(t), e.newAccount(t)
	e.fund(t, from, 1000, money.ARS)

	// The first failing value: 1 more than available.
	_, err := e.post(t.Context(), transfer(from, to, 1001, money.ARS, ledger.KindInternalTransfer))
	if !apperr.IsCode(err, apperr.CodeInsufficientFunds) {
		t.Fatalf("debit 1001 of 1000: err = %v, want ledger.insufficient_funds", err)
	}
	// The last passing value: exactly the available balance.
	if _, err := e.post(t.Context(), transfer(from, to, 1000, money.ARS, ledger.KindInternalTransfer)); err != nil {
		t.Fatalf("debit 1000 of 1000: %v", err)
	}
	assertBalance(t, e.balance(t, from, money.ARS), 0, 0, 0)
	assertBalance(t, e.balance(t, to, money.ARS), 1000, 0, 1000)
	// USD is a separate balance: ARS money never pays a USD debit.
	_, err = e.post(t.Context(), transfer(to, from, 1, money.USD, ledger.KindInternalTransfer))
	if !apperr.IsCode(err, apperr.CodeInsufficientFunds) {
		t.Fatalf("USD debit with only ARS: err = %v, want ledger.insufficient_funds", err)
	}
	assertLedgerSumsToZero(t, e.pool)
}

func TestPostSameFactTwiceIsRejected(t *testing.T) {
	t.Parallel()
	e := setup(t)
	acc := e.newAccount(t)
	entry := transfer(e.master, acc, 500, money.ARS, ledger.KindPayin)
	if _, err := e.post(t.Context(), entry); err != nil {
		t.Fatal(err)
	}
	if _, err := e.post(t.Context(), entry); !errors.Is(err, ledger.ErrDuplicateRef) {
		t.Fatalf("second post of the same payin: err = %v, want ErrDuplicateRef", err)
	}
	// A reversal of the same fact is another kind, so it posts.
	reversal := transfer(acc, e.master, 500, money.ARS, ledger.KindReversal)
	reversal.RefType, reversal.RefID = entry.RefType, entry.RefID
	if _, err := e.post(t.Context(), reversal); err != nil {
		t.Fatalf("reversal: %v", err)
	}
	assertBalance(t, e.balance(t, acc, money.ARS), 0, 0, 0)
}

func TestDatabaseEnforcesBalancedAndAppendOnlyPostings(t *testing.T) {
	t.Parallel()
	e := setup(t)
	acc := e.newAccount(t)
	var ledgerAccount uuid.UUID
	if err := e.pool.QueryRow(t.Context(), `SELECT id FROM ledger_accounts WHERE account_id = $1 AND currency = 'ARS'`, acc).Scan(&ledgerAccount); err != nil {
		t.Fatal(err)
	}
	// Code that bypasses Post cannot commit a one-sided transaction (deferred trigger).
	err := db.WithTx(t.Context(), e.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		txID := ids.New()
		if _, err := tx.Exec(t.Context(), `INSERT INTO ledger_transactions (id, kind, ref_type, ref_id)
			VALUES ($1, 'adjustment', 'test', $2)`, txID, ids.New()); err != nil {
			return err
		}
		_, err := tx.Exec(t.Context(), `INSERT INTO ledger_postings (id, transaction_id, ledger_account_id, currency, amount_minor)
			VALUES ($1, $2, $3, 'ARS', 100)`, ids.New(), txID, ledgerAccount)
		return err
	})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("unbalanced commit: err = %v, want check_violation 23514", err)
	}

	txID := e.postOK(t, transfer(e.master, acc, 100, money.ARS, ledger.KindPayin))
	for _, stmt := range []string{
		`UPDATE ledger_postings SET amount_minor = 1 WHERE transaction_id = $1`,
		`DELETE FROM ledger_postings WHERE transaction_id = $1`,
		`DELETE FROM ledger_transactions WHERE id = $1`,
	} {
		if _, err := e.pool.Exec(t.Context(), stmt, txID); err == nil {
			t.Fatalf("%s: succeeded on an append-only table", stmt)
		}
	}
}

func (e env) postOK(t *testing.T, entry ledger.Entry) uuid.UUID {
	t.Helper()
	id, err := e.post(t.Context(), entry)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// N concurrent debits of 300 against 1000: exactly 3 succeed, the balance ends at 100 and never
// goes below zero (the lock serializes them; READ COMMITTED proves the lock, not a retry).
func TestConcurrentDebitsNeverOverdraw(t *testing.T) {
	t.Parallel()
	e := setup(t)
	from, to := e.newAccount(t), e.newAccount(t)
	e.fund(t, from, 1000, money.ARS)

	const workers = 8
	var (
		wg               sync.WaitGroup
		mu               sync.Mutex
		ok, insufficient int
		other            []error
	)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.post(t.Context(), transfer(from, to, 300, money.ARS, ledger.KindInternalTransfer))
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case apperr.IsCode(err, apperr.CodeInsufficientFunds):
				insufficient++
			default:
				other = append(other, err)
			}
		}()
	}
	wg.Wait()
	if len(other) > 0 || ok != 3 || insufficient != workers-3 {
		t.Fatalf("ok = %d, insufficient = %d, other = %v; want 3, %d, none", ok, insufficient, other, workers-3)
	}
	assertBalance(t, e.balance(t, from, money.ARS), 100, 0, 100)
	assertBalance(t, e.balance(t, to, money.ARS), 900, 0, 900)
	assertLedgerSumsToZero(t, e.pool)
}

// The same race through the serializable helper every money caller uses: never an overdraft,
// and every failure is a known code (insufficient funds or in progress after the retries).
func TestConcurrentSerializableDebitsNeverOverdraw(t *testing.T) {
	t.Parallel()
	e := setup(t)
	from, to := e.newAccount(t), e.newAccount(t)
	e.fund(t, from, 1000, money.ARS)

	const workers = 8
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		ok    int
		other []error
	)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := db.WithSerializable(t.Context(), e.pool, func(tx pgx.Tx) error {
				_, err := e.svc.Post(t.Context(), tx, transfer(from, to, 300, money.ARS, ledger.KindInternalTransfer))
				return err
			})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case apperr.IsCode(err, apperr.CodeInsufficientFunds), apperr.IsCode(err, apperr.CodeOperationInProgress):
			default:
				other = append(other, err)
			}
		}()
	}
	wg.Wait()
	if len(other) > 0 || ok < 1 || ok > 3 {
		t.Fatalf("ok = %d, unexpected errors = %v; want 1..3 and none", ok, other)
	}
	assertBalance(t, e.balance(t, from, money.ARS), 1000-300*int64(ok), 0, 1000-300*int64(ok))
	assertLedgerSumsToZero(t, e.pool)
}

// Transfers in both directions between the same pair lock in one global order: no deadlock.
func TestOpposingTransfersDoNotDeadlock(t *testing.T) {
	t.Parallel()
	e := setup(t)
	a, b := e.newAccount(t), e.newAccount(t)
	e.fund(t, a, 10_000, money.ARS)
	e.fund(t, b, 10_000, money.ARS)

	const rounds = 8
	errs := make(chan error, 2*rounds)
	var wg sync.WaitGroup
	for range rounds {
		for _, pair := range [][2]uuid.UUID{{a, b}, {b, a}} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := e.post(t.Context(), transfer(pair[0], pair[1], 100, money.ARS, ledger.KindInternalTransfer))
				errs <- err
			}()
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("opposing transfer failed: %v", err)
		}
	}
	assertBalance(t, e.balance(t, a, money.ARS), 10_000, 0, 10_000)
	assertBalance(t, e.balance(t, b, money.ARS), 10_000, 0, 10_000)
}

func (e env) hold(ctx context.Context, account uuid.UUID, minor int64) (uuid.UUID, error) {
	var holdID uuid.UUID
	err := db.WithTx(ctx, e.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var err error
		holdID, err = e.svc.Hold(ctx, tx, ledger.HoldInput{
			AccountID: account, Amount: money.New(minor, money.ARS), RefType: "payout", RefID: ids.New(),
		})
		return err
	})
	return holdID, err
}

func (e env) inTx(t *testing.T, fn func(tx pgx.Tx) error) error {
	t.Helper()
	return db.WithTx(t.Context(), e.pool, pgx.TxOptions{}, fn)
}

func TestHoldLifecycle(t *testing.T) {
	t.Parallel()
	e := setup(t)
	acc := e.newAccount(t)
	e.fund(t, acc, 1000, money.ARS)
	ctx := t.Context()

	first, err := e.hold(ctx, acc, 600)
	if err != nil {
		t.Fatal(err)
	}
	assertBalance(t, e.balance(t, acc, money.ARS), 1000, 600, 400)

	if _, err := e.hold(ctx, acc, 401); !apperr.IsCode(err, apperr.CodeInsufficientFunds) {
		t.Fatalf("hold 401 of 400 available: err = %v, want ledger.insufficient_funds", err)
	}
	second, err := e.hold(ctx, acc, 400) // the last passing value
	if err != nil {
		t.Fatalf("hold 400 of 400 available: %v", err)
	}
	assertBalance(t, e.balance(t, acc, money.ARS), 1000, 1000, 0)

	// Held money cannot leave through a plain posting either.
	if _, err := e.post(ctx, transfer(acc, e.master, 1, money.ARS, ledger.KindAdjustment)); !apperr.IsCode(err, apperr.CodeInsufficientFunds) {
		t.Fatalf("debit with everything held: err = %v, want ledger.insufficient_funds", err)
	}

	if err := e.inTx(t, func(tx pgx.Tx) error { return e.svc.ReleaseHold(ctx, tx, second) }); err != nil {
		t.Fatal(err)
	}
	assertBalance(t, e.balance(t, acc, money.ARS), 1000, 600, 400)
	if err := e.inTx(t, func(tx pgx.Tx) error { return e.svc.ReleaseHold(ctx, tx, second) }); !apperr.IsCode(err, apperr.CodeHoldNotActive) {
		t.Fatalf("second release: err = %v, want ledger.hold_not_active", err)
	}

	// Capture: the payout leaves (600 = 550 to the PSP + 50 fee) and the hold is consumed.
	capture := ledger.Entry{
		Kind: ledger.KindPayout, RefType: "payout", RefID: ids.New(),
		Legs: []ledger.Leg{
			{AccountID: acc, Amount: money.New(-600, money.ARS)},
			{AccountID: e.master, Amount: money.New(550, money.ARS)},
			{AccountID: e.fees, Amount: money.New(50, money.ARS)},
		},
	}
	if err := e.inTx(t, func(tx pgx.Tx) error { _, err := e.svc.CaptureHold(ctx, tx, first, capture); return err }); err != nil {
		t.Fatal(err)
	}
	assertBalance(t, e.balance(t, acc, money.ARS), 400, 0, 400)
	assertBalance(t, e.balance(t, e.fees, money.ARS), 50, 0, 50)
	if err := e.inTx(t, func(tx pgx.Tx) error { return e.svc.ReleaseHold(ctx, tx, first) }); !apperr.IsCode(err, apperr.CodeHoldNotActive) {
		t.Fatalf("release after capture: err = %v, want ledger.hold_not_active", err)
	}
	if err := e.inTx(t, func(tx pgx.Tx) error { return e.svc.ReleaseHold(ctx, tx, ids.New()) }); !apperr.IsCode(err, apperr.CodeHoldNotActive) {
		t.Fatalf("release of a missing hold: err = %v, want ledger.hold_not_active", err)
	}
	var statuses string
	if err := e.pool.QueryRow(ctx, `SELECT string_agg(status, ',' ORDER BY status) FROM holds`).Scan(&statuses); err != nil {
		t.Fatal(err)
	}
	if statuses != "captured,released" {
		t.Fatalf("hold statuses = %q, want captured,released", statuses)
	}
	assertLedgerSumsToZero(t, e.pool)
}

func TestCaptureMustConsumeTheHoldExactly(t *testing.T) {
	t.Parallel()
	e := setup(t)
	acc, other := e.newAccount(t), e.newAccount(t)
	e.fund(t, acc, 1000, money.ARS)
	e.fund(t, other, 1000, money.ARS)
	holdID, err := e.hold(t.Context(), acc, 500)
	if err != nil {
		t.Fatal(err)
	}
	for name, entry := range map[string]ledger.Entry{
		"less than held":   transfer(acc, e.master, 499, money.ARS, ledger.KindPayout),
		"more than held":   transfer(acc, e.master, 501, money.ARS, ledger.KindPayout),
		"another account":  transfer(other, e.master, 500, money.ARS, ledger.KindPayout),
		"another currency": transfer(acc, e.master, 500, money.USD, ledger.KindPayout),
	} {
		err := e.inTx(t, func(tx pgx.Tx) error { _, err := e.svc.CaptureHold(t.Context(), tx, holdID, entry); return err })
		if !errors.Is(err, ledger.ErrCaptureMismatch) {
			t.Fatalf("%s: err = %v, want ErrCaptureMismatch", name, err)
		}
	}
	assertBalance(t, e.balance(t, acc, money.ARS), 1000, 500, 500)
}

func TestHoldRejectsBadInputAndDuplicates(t *testing.T) {
	t.Parallel()
	e := setup(t)
	acc := e.newAccount(t)
	e.fund(t, acc, 1000, money.ARS)
	ref := ids.New()
	hold := func(minor int64, c money.Currency) error {
		return e.inTx(t, func(tx pgx.Tx) error {
			_, err := e.svc.Hold(t.Context(), tx, ledger.HoldInput{AccountID: acc, Amount: money.New(minor, c), RefType: "payout", RefID: ref})
			return err
		})
	}
	if err := hold(0, money.ARS); !apperr.IsCode(err, apperr.CodeInvalidAmount) {
		t.Fatalf("hold 0: err = %v, want money.invalid_amount", err)
	}
	if err := hold(-5, money.ARS); !apperr.IsCode(err, apperr.CodeInvalidAmount) {
		t.Fatalf("hold -5: err = %v, want money.invalid_amount", err)
	}
	if err := hold(5, "EUR"); !apperr.IsCode(err, apperr.CodeCurrencyMismatch) {
		t.Fatalf("hold EUR: err = %v, want money.currency_mismatch", err)
	}
	if err := hold(1, money.ARS); err != nil {
		t.Fatal(err)
	}
	if err := hold(1, money.ARS); !errors.Is(err, ledger.ErrDuplicateRef) {
		t.Fatalf("second hold of the same payout: err = %v, want ErrDuplicateRef", err)
	}
}

func TestConcurrentHoldsNeverExceedAvailable(t *testing.T) {
	t.Parallel()
	e := setup(t)
	acc := e.newAccount(t)
	e.fund(t, acc, 1000, money.ARS)
	const workers = 8
	results := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.hold(t.Context(), acc, 300)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	ok := 0
	for err := range results {
		switch {
		case err == nil:
			ok++
		case !apperr.IsCode(err, apperr.CodeInsufficientFunds):
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if ok != 3 {
		t.Fatalf("holds placed = %d, want 3", ok)
	}
	assertBalance(t, e.balance(t, acc, money.ARS), 1000, 900, 100)
}

func TestLockBalancesReadsInsideTheTransaction(t *testing.T) {
	t.Parallel()
	e := setup(t)
	acc := e.newAccount(t)
	e.fund(t, acc, 700, money.USD)
	err := e.inTx(t, func(tx pgx.Tx) error {
		got, err := e.svc.LockBalances(t.Context(), tx, []uuid.UUID{acc})
		if err != nil {
			return err
		}
		if len(got) != 2 {
			t.Fatalf("balances = %d, want 2 (ARS, USD)", len(got))
		}
		for _, b := range got {
			want := int64(0)
			if b.Posted.Currency == money.USD {
				want = 700
			}
			if b.AccountID != acc || b.Posted.Minor != want || b.Available.Minor != want {
				t.Fatalf("balance %+v, want %d %s", b, want, b.Posted.Currency)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
