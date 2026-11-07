//go:build integration

package movements_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/ledger"
	"github.com/erfajc97/lenapay/apps/api/internal/movements"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db/dbtest"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/page"
)

var pg *dbtest.Postgres

func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }

// Business time zone UTC-3: 2026-11-08 02:30 UTC is still Nov 7 for the user.
func buenosAires(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

type env struct {
	pool          *pgxpool.Pool
	clk           *clock.Fake
	ledger        ledger.Service
	svc           movements.Service
	p             auth.Principal
	user          uuid.UUID
	main, project uuid.UUID
	cvu           uuid.UUID
}

func setup(t *testing.T) env {
	t.Helper()
	pool := pg.NewDatabase(t)
	clk := clock.NewFake(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	led := ledger.NewService(ledger.Deps{Pool: pool, Clock: clk})
	if err := led.EnsureSystemAccounts(t.Context()); err != nil {
		t.Fatal(err)
	}
	e := env{pool: pool, clk: clk, ledger: led, svc: movements.NewService(movements.Deps{Pool: pool, Location: buenosAires(t)})}
	e.user = ids.New()
	org := ids.New()
	e.exec(t, `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, 'x')`, e.user, e.user.String()+"@t.local")
	e.exec(t, `INSERT INTO organizations (id, kind, legal_name, kyc_status, created_by) VALUES ($1, 'company', 'Mov SA', 'approved', $2)`, org, e.user)
	e.p = auth.Principal{UserID: e.user, OrgID: org, Role: auth.RoleViewer}
	e.main, e.project = e.account(t, org, "main", "Principal"), e.account(t, org, "project", "Torre")
	e.cvu = ids.New()
	e.exec(t, `INSERT INTO cvus (id, org_id, account_id, currency, cvu, psp_ref) VALUES ($1, $2, $3, 'ARS', $4, $5)`,
		e.cvu, org, e.main, "0000000000000000000001", "psp-"+e.cvu.String())
	return e
}

func (e env) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := e.pool.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func (e env) account(t *testing.T, org uuid.UUID, kind, name string) uuid.UUID {
	t.Helper()
	id := ids.New()
	e.exec(t, `INSERT INTO accounts (id, org_id, kind, name) VALUES ($1, $2, $3, $4)`, id, org, kind, name)
	if err := db.WithTx(t.Context(), e.pool, pgx.TxOptions{}, func(tx pgx.Tx) error { return e.ledger.OpenAccount(t.Context(), tx, id) }); err != nil {
		t.Fatal(err)
	}
	return id
}

func (e env) payin(t *testing.T, at time.Time, status string, gross, fee int64) uuid.UUID {
	t.Helper()
	id := ids.New()
	var ledgerTx *uuid.UUID
	var creditedAt *time.Time
	if status == "credited" { // a credited payin points at its ledger transaction (DB CHECK)
		master, err := e.ledger.SystemAccountID(t.Context(), ledger.SystemPSPMaster)
		if err != nil {
			t.Fatal(err)
		}
		err = db.WithTx(t.Context(), e.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
			txID, err := e.ledger.Post(t.Context(), tx, ledger.Entry{
				Kind: ledger.KindPayin, RefType: "payin", RefID: id, OrgID: &e.p.OrgID,
				Legs: []ledger.Leg{{AccountID: master, Amount: money.New(-gross, money.ARS)}, {AccountID: e.main, Amount: money.New(gross, money.ARS)}},
			})
			ledgerTx = &txID
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		creditedAt = &at
	}
	e.exec(t, `INSERT INTO payins (id, org_id, account_id, cvu_id, currency, gross_minor, fee_minor, net_minor,
		origin_name, origin_cuit, origin_cbu, provider_transfer_id, status, created_at, ledger_transaction_id, credited_at)
		VALUES ($1, $2, $3, $4, 'ARS', $5, $6, $7, 'Comprador Uno', '20123456789', '0170000000000000000001', $8, $9, $10, $11, $12)`,
		id, e.p.OrgID, e.main, e.cvu, gross, fee, gross-fee, "tr-"+id.String(), status, at, ledgerTx, creditedAt)
	return id
}

func (e env) payout(t *testing.T, at time.Time, status string, currency money.Currency, account uuid.UUID) uuid.UUID {
	t.Helper()
	id := ids.New()
	e.exec(t, `INSERT INTO payouts (id, org_id, account_id, currency, amount_minor, fee_minor, destination_type,
		destination_value, destination_account, destination_holder_name, destination_holder_cuit, concept, status,
		created_by, idempotency_key, created_at)
		VALUES ($1, $2, $3, $4, 5000, 150, 'cbu', '0110000000000000000002', '0110000000000000000002', 'Proveedor SRL',
		        '30712345678', 'Cement order', $5, $6, $7, $8)`,
		id, e.p.OrgID, account, string(currency), status, e.user, "payout-key-"+id.String(), at)
	return id
}

// transfer posts an internal transfer at `at` (two movements: out and in).
func (e env) transfer(t *testing.T, at time.Time, from, to uuid.UUID, minor int64) uuid.UUID {
	t.Helper()
	e.clk.Set(at)
	master, err := e.ledger.SystemAccountID(t.Context(), ledger.SystemPSPMaster)
	if err != nil {
		t.Fatal(err)
	}
	ref := ids.New()
	err = db.WithTx(t.Context(), e.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if _, err := e.ledger.Post(t.Context(), tx, ledger.Entry{ // funding: a ledger payin, not a movement row
			Kind: ledger.KindPayin, RefType: "payin", RefID: ids.New(),
			Legs: []ledger.Leg{{AccountID: master, Amount: money.New(-minor, money.ARS)}, {AccountID: from, Amount: money.New(minor, money.ARS)}},
		}); err != nil {
			return err
		}
		_, err := e.ledger.Post(t.Context(), tx, ledger.Entry{
			Kind: ledger.KindInternalTransfer, RefType: "internal_transfer", RefID: ref, OrgID: &e.p.OrgID, Memo: "Aporte",
			Legs: []ledger.Leg{{AccountID: from, Amount: money.New(-minor, money.ARS)}, {AccountID: to, Amount: money.New(minor, money.ARS)}},
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func (e env) list(t *testing.T, f movements.Filter) page.Page[movements.Movement] {
	t.Helper()
	if f.Page.Limit == 0 {
		f.Page.Limit = page.MaxLimit
	}
	got, err := e.svc.List(t.Context(), e.p, f)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func refs(items []movements.Movement) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(items))
	for _, m := range items {
		out = append(out, m.RefID)
	}
	return out
}

func TestListMapsEveryKindNewestFirst(t *testing.T) {
	t.Parallel()
	e := setup(t)
	day := time.Date(2026, 11, 7, 13, 0, 0, 0, time.UTC)
	in := e.payin(t, day, "credited", 10_000, 50)
	out := e.payout(t, day.Add(time.Hour), "pending_approval", money.ARS, e.main)
	tr := e.transfer(t, day.Add(2*time.Hour), e.main, e.project, 700)

	// Another org's movements never show (T2).
	otherUser, otherOrg := ids.New(), ids.New()
	e.exec(t, `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, 'x')`, otherUser, otherUser.String()+"@t.local")
	e.exec(t, `INSERT INTO organizations (id, kind, legal_name, created_by) VALUES ($1, 'company', 'Otra SA', $2)`, otherOrg, otherUser)
	foreignAcc := e.account(t, otherOrg, "project", "Ajena")
	e.exec(t, `INSERT INTO payouts (id, org_id, account_id, currency, amount_minor, fee_minor, destination_type,
		destination_value, destination_account, destination_holder_name, destination_holder_cuit, concept, status, created_by, idempotency_key)
		VALUES ($1, $2, $3, 'ARS', 1, 0, 'cbu', '0110000000000000000002', '0110000000000000000002', 'X', '30712345678', 'x', 'settled', $4, 'foreign-key-00000001')`,
		ids.New(), otherOrg, foreignAcc, otherUser)

	got := e.list(t, movements.Filter{}).Items
	if len(got) != 4 {
		t.Fatalf("movements = %d, want 4 (payin, payout, transfer out + in)", len(got))
	}
	byRef := map[uuid.UUID][]movements.Movement{}
	for _, m := range got {
		byRef[m.RefID] = append(byRef[m.RefID], m)
	}
	pi := byRef[in][0]
	if pi.Type != movements.TypePayin || pi.Status != movements.StatusCompleted || pi.Direction != movements.DirectionIn ||
		pi.Amount != money.New(10_000, money.ARS) || pi.Fee == nil || *pi.Fee != money.New(50, money.ARS) ||
		pi.Counterparty.Name == nil || *pi.Counterparty.Name != "Comprador Uno" || *pi.Counterparty.CUIT != "20123456789" {
		t.Fatalf("payin movement = %+v", pi)
	}
	po := byRef[out][0]
	if po.Type != movements.TypePayout || po.Status != movements.StatusPending || po.Direction != movements.DirectionOut ||
		po.Amount != money.New(5000, money.ARS) || *po.Fee != money.New(150, money.ARS) || *po.Concept != "Cement order" ||
		*po.Counterparty.CBU != "0110000000000000000002" {
		t.Fatalf("payout movement = %+v", po)
	}
	legs := byRef[tr]
	if len(legs) != 2 {
		t.Fatalf("transfer legs = %d, want 2", len(legs))
	}
	for _, leg := range legs {
		wantDir, wantAcc, wantOther := movements.DirectionOut, e.main, "Torre"
		if leg.AccountID == e.project {
			wantDir, wantAcc, wantOther = movements.DirectionIn, e.project, "Principal"
		}
		if leg.Type != movements.TypeInternalTransfer || leg.Direction != wantDir || leg.AccountID != wantAcc ||
			leg.Amount != money.New(700, money.ARS) || leg.Fee != nil || *leg.Concept != "Aporte" || *leg.Counterparty.Name != wantOther {
			t.Fatalf("transfer leg = %+v", leg)
		}
	}
	for i := 1; i < len(got); i++ {
		if got[i].CreatedAt.After(got[i-1].CreatedAt) {
			t.Fatalf("not newest first at %d: %v after %v", i, got[i].CreatedAt, got[i-1].CreatedAt)
		}
	}
}

func TestFilters(t *testing.T) {
	t.Parallel()
	e := setup(t)
	nov7Late := time.Date(2026, 11, 8, 2, 30, 0, 0, time.UTC) // 23:30 on Nov 7 in Buenos Aires
	nov8 := time.Date(2026, 11, 8, 15, 0, 0, 0, time.UTC)
	credited := e.payin(t, nov7Late, "credited", 1000, 5)
	received := e.payin(t, nov8, "received", 2000, 10)
	settledUSD := e.payout(t, nov8, "settled", money.USD, e.project)
	rejected := e.payout(t, nov8.Add(time.Minute), "rejected", money.ARS, e.main)

	day := func(y int, m time.Month, d int) *time.Time { v := time.Date(y, m, d, 0, 0, 0, 0, time.UTC); return &v }
	typ := func(v movements.Type) *movements.Type { return &v }
	status := func(v movements.Status) *movements.Status { return &v }
	cur := func(v money.Currency) *money.Currency { return &v }

	cases := []struct {
		name string
		f    movements.Filter
		want []uuid.UUID
	}{
		{"type payin", movements.Filter{Type: typ(movements.TypePayin)}, []uuid.UUID{received, credited}},
		{"status pending", movements.Filter{Status: status(movements.StatusPending)}, []uuid.UUID{received}},
		{"status cancelled maps rejected payouts", movements.Filter{Status: status(movements.StatusCancelled)}, []uuid.UUID{rejected}},
		{"currency USD", movements.Filter{Currency: cur(money.USD)}, []uuid.UUID{settledUSD}},
		{"account project", movements.Filter{AccountID: &e.project}, []uuid.UUID{settledUSD}},
		{"from Nov 8 excludes 23:30 of Nov 7 local", movements.Filter{From: day(2026, 11, 8)}, []uuid.UUID{rejected, settledUSD, received}},
		{"to Nov 7 includes 23:30 local (02:30 UTC Nov 8)", movements.Filter{To: day(2026, 11, 7)}, []uuid.UUID{credited}},
		{"single day Nov 8", movements.Filter{From: day(2026, 11, 8), To: day(2026, 11, 8)}, []uuid.UUID{rejected, settledUSD, received}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := refs(e.list(t, tc.f).Items)
			if len(got) != len(tc.want) {
				t.Fatalf("refs = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("refs = %v, want %v", got, tc.want)
				}
			}
		})
	}
	_, err := e.svc.List(t.Context(), e.p, movements.Filter{From: day(2026, 11, 9), To: day(2026, 11, 8), Page: page.Request{Limit: 10}})
	if !apperr.IsCode(err, apperr.CodeValidation) {
		t.Fatalf("from after to: err = %v, want validation.failed", err)
	}
}

func TestCursorPagination(t *testing.T) {
	t.Parallel()
	e := setup(t)
	base := time.Date(2026, 11, 7, 13, 0, 0, 0, time.UTC)
	var want []uuid.UUID
	for i := range 5 {
		// Two rows share each instant, so the id tiebreaker decides the order.
		want = append(want, e.payin(t, base.Add(time.Duration(i/2)*time.Minute), "credited", 100+int64(i), 0))
	}
	seen := map[uuid.UUID]bool{}
	var cursor *page.Position
	pages := 0
	for {
		got := e.list(t, movements.Filter{Page: page.Request{Limit: 2, After: cursor}})
		pages++
		for _, m := range got.Items {
			if seen[m.ID] {
				t.Fatalf("movement %s returned twice", m.ID)
			}
			seen[m.ID] = true
		}
		if got.NextCursor == nil {
			if len(got.Items) != 1 {
				t.Fatalf("last page has %d items, want 1", len(got.Items))
			}
			break
		}
		pos, err := page.Decode(*got.NextCursor)
		if err != nil {
			t.Fatal(err)
		}
		cursor = &pos
	}
	if pages != 3 || len(seen) != len(want) {
		t.Fatalf("pages = %d, seen = %d; want 3 pages and %d movements", pages, len(seen), len(want))
	}
}
