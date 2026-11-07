//go:build integration

package accounts_test

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/accounts"
	"github.com/erfajc97/lenapay/apps/api/internal/ledger"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/audit"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db/dbtest"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/idempotency"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
)

var pg *dbtest.Postgres

func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }

type env struct {
	pool   *pgxpool.Pool
	svc    accounts.Service
	ledger ledger.Service
	p      auth.Principal // admin of org A with a fresh step-up
	other  auth.Principal // admin of org B
}

func setup(t *testing.T) env {
	t.Helper()
	pool := pg.NewDatabase(t)
	clk := clock.NewFake(time.Date(2026, 11, 7, 15, 0, 0, 0, time.UTC))
	led := ledger.NewService(ledger.Deps{Pool: pool, Clock: clk})
	if err := led.EnsureSystemAccounts(t.Context()); err != nil {
		t.Fatal(err)
	}
	svc := accounts.NewService(accounts.Deps{
		Pool: pool, Clock: clk, Audit: audit.NewLogger(clk),
		Idempotency: idempotency.NewStore(clk, idempotency.DefaultTTL), Ledger: led,
	})
	e := env{pool: pool, svc: svc, ledger: led}
	e.p = e.newOrg(t, "company")
	e.other = e.newOrg(t, "company")
	return e
}

func (e env) newOrg(t *testing.T, kind string) auth.Principal {
	t.Helper()
	user, org := ids.New(), ids.New()
	e.exec(t, `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, 'x')`, user, user.String()+"@t.local")
	e.exec(t, `INSERT INTO organizations (id, kind, legal_name, kyc_status, created_by) VALUES ($1, $2, 'Org SA', 'approved', $3)`, org, kind, user)
	return auth.Principal{UserID: user, SessionID: ids.New(), OrgID: org, Role: auth.RoleAdmin, KYCStatus: auth.KYCApproved, StepUp: true}
}

func (e env) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := e.pool.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func (e env) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(t.Context(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

func (e env) project(t *testing.T, p auth.Principal, name string) accounts.Account {
	t.Helper()
	acc, err := e.svc.Create(t.Context(), p, accounts.CreateInput{Name: name})
	if err != nil {
		t.Fatal(err)
	}
	return acc
}

// fund credits an account like a payin (psp_master → account).
func (e env) fund(t *testing.T, account uuid.UUID, minor int64, c money.Currency) {
	t.Helper()
	master, err := e.ledger.SystemAccountID(t.Context(), ledger.SystemPSPMaster)
	if err != nil {
		t.Fatal(err)
	}
	err = db.WithTx(t.Context(), e.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err := e.ledger.Post(t.Context(), tx, ledger.Entry{
			Kind: ledger.KindPayin, RefType: "payin", RefID: ids.New(),
			Legs: []ledger.Leg{{AccountID: master, Amount: money.New(-minor, c)}, {AccountID: account, Amount: money.New(minor, c)}},
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func available(t *testing.T, acc accounts.Account, c money.Currency) int64 {
	t.Helper()
	for _, b := range acc.Balances {
		if b.Posted.Currency == c {
			return b.Available.Minor
		}
	}
	t.Fatalf("account %s has no %s balance", acc.ID, c)
	return 0
}

func (e env) get(t *testing.T, id uuid.UUID) accounts.Account {
	t.Helper()
	acc, err := e.svc.Get(t.Context(), e.p.OrgID, id)
	if err != nil {
		t.Fatal(err)
	}
	return acc
}

func TestOnKYCApprovedOpensOneMainOrPersonalAccount(t *testing.T) {
	t.Parallel()
	e := setup(t)
	person := e.newOrg(t, "personal")
	for _, tc := range []struct {
		p    auth.Principal
		kind string
		want accounts.Kind
	}{{e.p, "company", accounts.KindMain}, {person, "personal", accounts.KindPersonal}} {
		for range 2 { // the hook may run again: still one account
			if err := db.WithTx(t.Context(), e.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
				return e.svc.OnKYCApproved(t.Context(), tx, tc.p.OrgID, tc.kind)
			}); err != nil {
				t.Fatal(err)
			}
		}
		list, err := e.svc.List(t.Context(), tc.p, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 1 || list[0].Kind != tc.want || len(list[0].Balances) != 2 {
			t.Fatalf("%s org accounts = %+v, want one %s account with ARS and USD", tc.kind, list, tc.want)
		}
	}
	if got := e.count(t, `SELECT count(*) FROM audit_log WHERE action = 'account.open'`); got != 2 {
		t.Fatalf("account.open audit rows = %d, want 2", got)
	}
	err := db.WithTx(t.Context(), e.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return e.svc.OnKYCApproved(t.Context(), tx, e.other.OrgID, "cooperative")
	})
	if err == nil {
		t.Fatal("unknown org kind: want an error")
	}
}

func TestCreateRenameAndNameCaps(t *testing.T) {
	t.Parallel()
	e := setup(t)
	acc := e.project(t, e.p, "  Torre Norte  ")
	if acc.Name != "Torre Norte" || acc.Kind != accounts.KindProject || acc.Archived || len(acc.Balances) != 2 {
		t.Fatalf("created = %+v", acc)
	}
	longest := strings.Repeat("ñ", 80) // 80 runes is the last valid length (DB CHECK counts chars)
	renamed, err := e.svc.Update(t.Context(), e.p, acc.ID, accounts.UpdateInput{Name: &longest})
	if err != nil || renamed.Name != longest {
		t.Fatalf("rename to 80 runes: %+v, %v", renamed, err)
	}
	for name, bad := range map[string]string{"blank": "   ", "81 runes": strings.Repeat("a", 81)} {
		if _, err := e.svc.Create(t.Context(), e.p, accounts.CreateInput{Name: bad}); !apperr.IsCode(err, apperr.CodeValidation) {
			t.Fatalf("create %s: err = %v, want validation.failed", name, err)
		}
	}
	if got := e.count(t, `SELECT count(*) FROM audit_log WHERE action IN ('account.create', 'account.update') AND target_id = $1`, acc.ID); got != 2 {
		t.Fatalf("audit rows = %d, want 2", got)
	}
}

func TestAccountsOfAnotherOrgAreNotFound(t *testing.T) {
	t.Parallel()
	e := setup(t)
	foreign := e.project(t, e.other, "Ajeno")
	name := "Mío"
	if _, err := e.svc.Get(t.Context(), e.p.OrgID, foreign.ID); !apperr.IsCode(err, apperr.CodeAccountNotFound) {
		t.Fatalf("get foreign: err = %v, want account.not_found", err)
	}
	if _, err := e.svc.Update(t.Context(), e.p, foreign.ID, accounts.UpdateInput{Name: &name}); !apperr.IsCode(err, apperr.CodeAccountNotFound) {
		t.Fatalf("update foreign: err = %v, want account.not_found", err)
	}
	list, err := e.svc.List(t.Context(), e.p, true)
	if err != nil || len(list) != 0 {
		t.Fatalf("org A sees %d accounts (%v), want 0", len(list), err)
	}
}

func TestArchiveOnlyEmptyProjectAccounts(t *testing.T) {
	t.Parallel()
	e := setup(t)
	acc, sink := e.project(t, e.p, "Obra"), e.project(t, e.p, "Destino")
	e.fund(t, acc.ID, 1, money.USD) // one cent in any currency blocks the archive
	yes, no := true, false
	if _, err := e.svc.Update(t.Context(), e.p, acc.ID, accounts.UpdateInput{Archived: &yes}); !apperr.IsCode(err, apperr.CodeAccountNotEmpty) {
		t.Fatalf("archive with balance: err = %v, want account.not_empty", err)
	}
	if _, err := e.svc.Transfer(t.Context(), e.p, accounts.TransferInput{
		FromAccountID: acc.ID, ToAccountID: sink.ID, Amount: money.New(1, money.USD), IdempotencyKey: "empty-the-account-0001",
	}); err != nil {
		t.Fatal(err)
	}
	archived, err := e.svc.Update(t.Context(), e.p, acc.ID, accounts.UpdateInput{Archived: &yes})
	if err != nil || !archived.Archived {
		t.Fatalf("archive empty: %+v, %v", archived, err)
	}
	if list, _ := e.svc.List(t.Context(), e.p, false); len(list) != 1 {
		t.Fatalf("active accounts = %d, want 1", len(list))
	}
	if list, _ := e.svc.List(t.Context(), e.p, true); len(list) != 2 {
		t.Fatalf("all accounts = %d, want 2", len(list))
	}
	if _, err := e.svc.Transfer(t.Context(), e.p, accounts.TransferInput{
		FromAccountID: sink.ID, ToAccountID: acc.ID, Amount: money.New(1, money.USD), IdempotencyKey: "into-archived-000001",
	}); !apperr.IsCode(err, apperr.CodeAccountArchived) {
		t.Fatalf("transfer into archived: err = %v, want account.archived", err)
	}
	if restored, err := e.svc.Update(t.Context(), e.p, acc.ID, accounts.UpdateInput{Archived: &no}); err != nil || restored.Archived {
		t.Fatalf("unarchive: %+v, %v", restored, err)
	}

	if err := db.WithTx(t.Context(), e.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return e.svc.OnKYCApproved(t.Context(), tx, e.p.OrgID, "company")
	}); err != nil {
		t.Fatal(err)
	}
	mainAcc, err := e.svc.List(t.Context(), e.p, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range mainAcc {
		if a.Kind == accounts.KindMain {
			if _, err := e.svc.Update(t.Context(), e.p, a.ID, accounts.UpdateInput{Archived: &yes}); !apperr.IsCode(err, apperr.CodeAccountNotArchivable) {
				t.Fatalf("archive main account: err = %v, want account.not_archivable", err)
			}
		}
	}
}

func TestTransferMovesMoneyOnceAndAudits(t *testing.T) {
	t.Parallel()
	e := setup(t)
	from, to := e.project(t, e.p, "Origen"), e.project(t, e.p, "Destino")
	e.fund(t, from.ID, 1000, money.ARS)
	in := accounts.TransferInput{
		FromAccountID: from.ID, ToAccountID: to.ID, Amount: money.New(400, money.ARS),
		Concept: "  Aporte  ", IdempotencyKey: "transfer-key-000000001",
	}
	first, err := e.svc.Transfer(t.Context(), e.p, in)
	if err != nil {
		t.Fatal(err)
	}
	if first.Concept == nil || *first.Concept != "Aporte" || first.Amount != money.New(400, money.ARS) {
		t.Fatalf("transfer = %+v", first)
	}
	replay, err := e.svc.Transfer(t.Context(), e.p, in)
	if err != nil {
		t.Fatal(err)
	}
	if replay.ID != first.ID || !replay.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("replay = %+v, want the stored %+v", replay, first)
	}
	if got := available(t, e.get(t, from.ID), money.ARS); got != 600 {
		t.Fatalf("origin available = %d, want 600 (moved once)", got)
	}
	if got := available(t, e.get(t, to.ID), money.ARS); got != 400 {
		t.Fatalf("destination available = %d, want 400", got)
	}
	if got := e.count(t, `SELECT count(*) FROM ledger_transactions WHERE kind = 'internal_transfer' AND ref_id = $1
		AND idempotency_key = $2 AND created_by = $3 AND org_id = $4`, first.ID, in.IdempotencyKey, e.p.UserID, e.p.OrgID); got != 1 {
		t.Fatalf("ledger transactions = %d, want 1", got)
	}
	if got := e.count(t, `SELECT count(*) FROM audit_log WHERE action = 'internal_transfer.create' AND target_id = $1
		AND idempotency_key = $2 AND actor_id = $3`, first.ID, in.IdempotencyKey, e.p.UserID); got != 1 {
		t.Fatalf("audit rows = %d, want 1", got)
	}

	changed := in
	changed.Amount = money.New(401, money.ARS)
	if _, err := e.svc.Transfer(t.Context(), e.p, changed); !apperr.IsCode(err, apperr.CodeIdempotencyReused) {
		t.Fatalf("same key, other amount: err = %v, want idempotency.key_reused", err)
	}
}

func TestTransferRejections(t *testing.T) {
	t.Parallel()
	e := setup(t)
	from, to := e.project(t, e.p, "Origen"), e.project(t, e.p, "Destino")
	foreign := e.project(t, e.other, "Ajeno")
	e.fund(t, from.ID, 500, money.ARS)
	base := accounts.TransferInput{FromAccountID: from.ID, ToAccountID: to.ID, Amount: money.New(500, money.ARS), IdempotencyKey: "rejections-key-000001"}
	noStepUp := e.p
	noStepUp.StepUp = false

	cases := []struct {
		name string
		p    auth.Principal
		mut  func(*accounts.TransferInput)
		code apperr.Code
	}{
		{"without step-up", noStepUp, func(*accounts.TransferInput) {}, apperr.CodeStepUpRequired},
		{"zero amount", e.p, func(in *accounts.TransferInput) { in.Amount = money.New(0, money.ARS) }, apperr.CodeInvalidAmount},
		{"unsupported currency", e.p, func(in *accounts.TransferInput) { in.Amount = money.New(1, "EUR") }, apperr.CodeCurrencyMismatch},
		{"same account", e.p, func(in *accounts.TransferInput) { in.ToAccountID = from.ID }, apperr.CodeTransferSameAccount},
		{"to another org", e.p, func(in *accounts.TransferInput) { in.ToAccountID = foreign.ID }, apperr.CodeAccountNotFound},
		{"from another org", e.p, func(in *accounts.TransferInput) { in.FromAccountID = foreign.ID }, apperr.CodeAccountNotFound},
		{"more than available", e.p, func(in *accounts.TransferInput) { in.Amount = money.New(501, money.ARS) }, apperr.CodeInsufficientFunds},
		{"other currency balance", e.p, func(in *accounts.TransferInput) { in.Amount = money.New(1, money.USD) }, apperr.CodeInsufficientFunds},
		{"concept over 140", e.p, func(in *accounts.TransferInput) { in.Concept = strings.Repeat("c", 141) }, apperr.CodeValidation},
		{"missing key", e.p, func(in *accounts.TransferInput) { in.IdempotencyKey = " " }, apperr.CodeValidation},
	}
	for _, tc := range cases {
		in := base
		tc.mut(&in)
		if _, err := e.svc.Transfer(t.Context(), tc.p, in); !apperr.IsCode(err, tc.code) {
			t.Fatalf("%s: err = %v, want %s", tc.name, err, tc.code)
		}
	}
	// A failed attempt rolled its key back: the same key now succeeds (the base body passes).
	if _, err := e.svc.Transfer(t.Context(), e.p, base); err != nil {
		t.Fatalf("base transfer after rejections: %v", err)
	}
	foreignNow, err := e.svc.Get(t.Context(), e.other.OrgID, foreign.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := available(t, foreignNow, money.ARS); got != 0 {
		t.Fatalf("foreign account balance = %d, want 0", got)
	}
}

// Concurrent transfers with different keys never overdraw; the same key concurrently moves
// money once.
func TestConcurrentTransfers(t *testing.T) {
	t.Parallel()
	e := setup(t)
	from, to := e.project(t, e.p, "Origen"), e.project(t, e.p, "Destino")
	e.fund(t, from.ID, 1000, money.ARS)

	const workers = 6
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.svc.Transfer(t.Context(), e.p, accounts.TransferInput{
				FromAccountID: from.ID, ToAccountID: to.ID, Amount: money.New(300, money.ARS),
				IdempotencyKey: "distinct-key-00000000" + string(rune('a'+i)),
			})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	ok := 0
	for err := range errs {
		switch {
		case err == nil:
			ok++
		case apperr.IsCode(err, apperr.CodeInsufficientFunds), apperr.IsCode(err, apperr.CodeOperationInProgress):
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	got := available(t, e.get(t, from.ID), money.ARS)
	if ok < 1 || ok > 3 || got != 1000-300*int64(ok) {
		t.Fatalf("successes = %d, origin available = %d; want 1..3 and 1000 - 300 × successes", ok, got)
	}

	same := accounts.TransferInput{FromAccountID: to.ID, ToAccountID: from.ID, Amount: money.New(100, money.ARS), IdempotencyKey: "same-key-concurrently-1"}
	results := make(chan accounts.Transfer, workers)
	failures := make(chan error, workers)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := e.svc.Transfer(t.Context(), e.p, same)
			if err != nil {
				failures <- err
				return
			}
			results <- res
		}()
	}
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if !apperr.IsCode(err, apperr.CodeOperationInProgress) {
			t.Fatalf("same key: unexpected error %v", err)
		}
	}
	var id uuid.UUID
	for res := range results {
		if id != uuid.Nil && res.ID != id {
			t.Fatalf("same key returned two transfers: %s and %s", id, res.ID)
		}
		id = res.ID
	}
	if got := e.count(t, `SELECT count(*) FROM ledger_transactions WHERE idempotency_key = $1`, same.IdempotencyKey); got != 1 {
		t.Fatalf("ledger transactions for one key = %d, want 1", got)
	}
}

func TestSummaryTotalsActiveAccounts(t *testing.T) {
	t.Parallel()
	e := setup(t)
	a, b := e.project(t, e.p, "A"), e.project(t, e.p, "B")
	e.fund(t, a.ID, 700, money.ARS)
	e.fund(t, b.ID, 300, money.ARS)
	e.fund(t, b.ID, 25, money.USD)
	e.project(t, e.other, "Otra org") // never counted
	sum, err := e.svc.Summary(t.Context(), e.p)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Accounts) != 2 || len(sum.Totals) != 2 {
		t.Fatalf("summary = %d accounts, %d totals; want 2, 2", len(sum.Accounts), len(sum.Totals))
	}
	want := map[money.Currency]int64{money.ARS: 1000, money.USD: 25}
	for _, tot := range sum.Totals {
		if tot.Posted.Minor != want[tot.Posted.Currency] || tot.Available.Minor != want[tot.Posted.Currency] || tot.Held.Minor != 0 {
			t.Fatalf("total %s = %+v, want %d", tot.Posted.Currency, tot, want[tot.Posted.Currency])
		}
	}
}

func TestImportProjectIsIdempotentPerERPRef(t *testing.T) {
	t.Parallel()
	e := setup(t)
	first, err := e.svc.ImportProject(t.Context(), e.p.OrgID, "Edificio Sur", "erp-project-42")
	if err != nil {
		t.Fatal(err)
	}
	again, err := e.svc.ImportProject(t.Context(), e.p.OrgID, "Edificio Sur (renombrado)", "erp-project-42")
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != first.ID || first.ERPProjectRef == nil || *first.ERPProjectRef != "erp-project-42" || first.Kind != accounts.KindProject {
		t.Fatalf("import = %+v then %+v, want the same project account", first, again)
	}
	other, err := e.svc.ImportProject(t.Context(), e.other.OrgID, "Edificio Sur", "erp-project-42")
	if err != nil || other.ID == first.ID {
		t.Fatalf("same ref in another org = %+v, %v; want its own account", other, err)
	}
	if _, err := e.svc.ImportProject(t.Context(), e.p.OrgID, "X", " "); !apperr.IsCode(err, apperr.CodeValidation) {
		t.Fatalf("blank ref: err = %v, want validation.failed", err)
	}
}
