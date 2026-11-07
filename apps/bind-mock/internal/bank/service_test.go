package bank

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newBank(t *testing.T, stateFile string) (*Bank, *clock) {
	t.Helper()
	clk := &clock{now: time.Date(2026, 11, 7, 15, 0, 0, 0, time.UTC)}
	ar, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(Options{
		StateFile: stateFile, Location: ar, PendingDelay: 20 * time.Second, Now: clk.Now,
		InitialBalanceMinor: map[string]int64{"ARS": 1_000_000_00, "USD": 1_000_00},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b, clk
}

func caja(t *testing.T, b *Bank, ref, currency string) Caja {
	t.Helper()
	c, created, err := b.CreateCaja(CreateCajaInput{Reference: ref, Currency: currency, Holder: Holder{Name: "Juan Pérez", CUIT: "20123456786"}})
	if err != nil || !created {
		t.Fatalf("create caja %s: %v (created %v)", ref, err, created)
	}
	return c
}

// Amounts travel as decimals; a third decimal or a sign must never be rounded into money.
func TestParseAndFormatMinor(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]int64{"1500": 150000, "1500.5": 150050, "1500.25": 150025, "0.01": 1, "0": 0} {
		if got, err := ParseMinor(in); err != nil || got != want {
			t.Errorf("ParseMinor(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "1.234", "-1", "1e3", "1.", ".5", "12a", "1234567890123456"} {
		if _, err := ParseMinor(in); !errors.Is(err, ErrInvalidAmount) {
			t.Errorf("ParseMinor(%q) err = %v, want ErrInvalidAmount", in, err)
		}
	}
	if FormatMinor(150025) != "1500.25" || FormatMinor(5) != "0.05" || FormatMinor(-150) != "-1.50" {
		t.Fatal("FormatMinor")
	}
}

// Every CVU we issue has valid check digits (the API and other banks validate them).
func TestIssuedCVUsAreValidAndUnique(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for n := int64(1); n < 500; n++ {
		cvu := newCVU(n)
		if !ValidAccountNumber(cvu) || !strings.HasPrefix(cvu, cvuPrefix) || seen[cvu] {
			t.Fatalf("cvu %d = %s", n, cvu)
		}
		seen[cvu] = true
		if !ValidAlias(newAlias(n)) {
			t.Fatalf("alias %d = %s", n, newAlias(n))
		}
	}
	good := AccountNumberFor("42")
	broken := good[:21] + string('0'+(good[21]-'0'+1)%10)
	if !ValidAccountNumber(good) || ValidAccountNumber(broken) || ValidAccountNumber(good[:21]) {
		t.Fatalf("checksum: %s %s", good, broken)
	}
	if c := cuit("20", "12345678"); len(c) != 11 || c != "20123456786" {
		t.Fatalf("cuit = %s", c)
	}
}

// BIND dedupes caja creation by our reference: a retried POST never issues a second CVU.
func TestCreateCajaIsIdempotentAndAliasesAreUnique(t *testing.T) {
	t.Parallel()
	b, _ := newBank(t, "")
	first := caja(t, b, "ref-1", "ARS")
	again, created, err := b.CreateCaja(CreateCajaInput{Reference: "ref-1", Currency: "USD", Holder: Holder{Name: "Otro"}})
	if err != nil || created || again.ID != first.ID || again.CVU != first.CVU {
		t.Fatalf("replay = %+v created=%v err=%v", again, created, err)
	}
	if _, _, err := b.CreateCaja(CreateCajaInput{Reference: "ref-2", Currency: "ARS", Holder: Holder{Name: "X"}, Alias: strings.ToUpper(first.Alias)}); !errors.Is(err, ErrAliasTaken) {
		t.Fatalf("alias reuse (case-insensitive) err = %v", err)
	}
	if _, _, err := b.CreateCaja(CreateCajaInput{Reference: "ref-3", Currency: "EUR", Holder: Holder{Name: "X"}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("EUR err = %v", err)
	}
	second := caja(t, b, "ref-4", "USD")
	if _, err := b.UpdateAlias(second.ID, first.Alias); !errors.Is(err, ErrAliasTaken) {
		t.Fatalf("update to a taken alias err = %v", err)
	}
	if got, err := b.UpdateAlias(second.ID, "torre.norte"); err != nil || got.Alias != "torre.norte" {
		t.Fatalf("update alias = %+v %v", got, err)
	}
	if err := b.DisableCaja(second.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := b.UpdateAlias(second.ID, "otro.alias"); !errors.Is(err, ErrCajaDisabled) {
		t.Fatalf("alias of a disabled caja err = %v", err)
	}
	if len(b.ActiveCVUs()) != 1 {
		t.Fatalf("active = %d", len(b.ActiveCVUs()))
	}
}

func TestResolveDestinations(t *testing.T) {
	t.Parallel()
	b, _ := newBank(t, "")
	ours := caja(t, b, "ref-1", "ARS")
	res, err := b.ResolveAccount(ours.CVU)
	if err != nil || res.Holder.Name != "Juan Pérez" || res.Type != "CVU" || res.Alias != ours.Alias {
		t.Fatalf("our CVU = %+v %v", res, err)
	}
	if res, err := b.ResolveAlias(ours.Alias); err != nil || res.AccountNumber != ours.CVU {
		t.Fatalf("our alias = %+v %v", res, err)
	}
	external := AccountNumberFor("123456")
	r1, err1 := b.ResolveAccount(external)
	r2, err2 := b.ResolveAccount(external)
	if err1 != nil || err2 != nil || r1 != r2 || r1.Holder.Name == "" || len(r1.Holder.CUIT) != 11 || r1.BankName != "Banco de Galicia" {
		t.Fatalf("external must be deterministic: %+v %+v", r1, r2)
	}
	if r, err := b.ResolveAlias("proveedor.sur"); err != nil || !ValidAccountNumber(r.AccountNumber) {
		t.Fatalf("external alias = %+v %v", r, err)
	}
	if _, err := b.ResolveAccount(AccountNumberFor("9999000000001")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("9999 account err = %v", err)
	}
	if _, err := b.ResolveAlias("noexiste.nadie"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("noexiste alias err = %v", err)
	}
	for _, bad := range []string{external[:21] + "x", "123", ""} {
		if _, err := b.ResolveAccount(bad); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid %q err = %v", bad, err)
		}
	}
	if _, err := b.ResolveAlias("a b"); !errors.Is(err, ErrInvalid) {
		t.Fatal("alias with a space must be invalid")
	}
	if err := b.DisableCaja(ours.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := b.ResolveAccount(ours.CVU); !errors.Is(err, ErrNotFound) {
		t.Fatalf("disabled CVU err = %v", err)
	}
}

// The cents of the amount script the outcome; the same origin_id never moves money twice.
func TestSendTransferOutcomes(t *testing.T) {
	t.Parallel()
	to := AccountNumberFor("777")
	cases := []struct {
		name        string
		amount      int64
		wantOutcome Outcome
		wantStatus  string
		wantStored  bool
		wantDebited bool
	}{
		{"ok", 100_00, OutcomeOK, StatusCompleted, true, true},
		{"rejected", 100_13, OutcomeRejected, StatusFailed, true, false},
		{"unavailable: nothing happened", 100_96, OutcomeUnavailable, "", false, false},
		{"pending", 100_97, OutcomePending, StatusInProgress, true, false},
		{"timeout after executing", 100_98, OutcomeTimeout, StatusCompleted, true, true},
		{"500 after executing", 100_99, OutcomeError, StatusCompleted, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, _ := newBank(t, "")
			res, err := b.SendTransfer(SendInput{OriginID: "o-1", ToAccount: to, Currency: "ARS", Amount: tc.amount, Concept: "VAR"})
			if err != nil || res.Outcome != tc.wantOutcome || res.Transfer.Status != tc.wantStatus {
				t.Fatalf("res = %+v err = %v", res, err)
			}
			_, getErr := b.GetTransfer("o-1")
			if (getErr == nil) != tc.wantStored {
				t.Fatalf("stored = %v, want %v", getErr == nil, tc.wantStored)
			}
			balance, _ := b.Balance("ARS")
			if debited := balance != 1_000_000_00; debited != tc.wantDebited {
				t.Fatalf("balance %d, debited want %v", balance, tc.wantDebited)
			}
		})
	}
}

func TestSendTransferDedupesFundsAndDestinations(t *testing.T) {
	t.Parallel()
	b, clk := newBank(t, "")
	to := AccountNumberFor("777")
	first, err := b.SendTransfer(SendInput{OriginID: "o-1", ToAccount: to, Currency: "ARS", Amount: 500_00})
	if err != nil {
		t.Fatal(err)
	}
	again, err := b.SendTransfer(SendInput{OriginID: "o-1", ToAccount: to, Currency: "ARS", Amount: 500_00})
	if err != nil || again.Transfer.ID != first.Transfer.ID {
		t.Fatalf("same origin_id must return the first transfer: %+v %v", again, err)
	}
	if bal, _ := b.Balance("ARS"); bal != 1_000_000_00-500_00 {
		t.Fatalf("balance = %d (debited twice?)", bal)
	}
	// USD master holds 1.000,00: the last covered amount passes, one cent more fails.
	if res, _ := b.SendTransfer(SendInput{OriginID: "o-2", ToAccount: to, Currency: "USD", Amount: 1_000_01}); res.Transfer.FailureCode != FailureNoFunds {
		t.Fatalf("over balance = %+v", res.Transfer)
	}
	if res, _ := b.SendTransfer(SendInput{OriginID: "o-3", ToAccount: to, Currency: "USD", Amount: 1_000_00}); res.Transfer.Status != StatusCompleted {
		t.Fatalf("exact balance = %+v", res.Transfer)
	}
	if res, _ := b.SendTransfer(SendInput{OriginID: "o-4", ToAlias: "noexiste.x", Currency: "ARS", Amount: 1_00}); res.Transfer.FailureCode != FailureNoSuchTarget {
		t.Fatalf("unknown destination = %+v", res.Transfer)
	}
	if _, err := b.SendTransfer(SendInput{OriginID: "o-5", ToAccount: "123", Currency: "ARS", Amount: 1_00}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("malformed destination err = %v", err)
	}
	b.SetNextOutcome(OutcomePending)
	pending, _ := b.SendTransfer(SendInput{OriginID: "o-6", ToAccount: to, Currency: "ARS", Amount: 10_00})
	clk.Advance(19 * time.Second) // one second before PendingDelay
	if got, _ := b.GetTransfer(pending.Transfer.ID); got.Status != StatusInProgress {
		t.Fatalf("settled early: %+v", got)
	}
	clk.Advance(time.Second)
	if got, _ := b.GetTransfer(pending.Transfer.ID); got.Status != StatusCompleted {
		t.Fatalf("not settled at PendingDelay: %+v", got)
	}
}

func TestIncomingAndStatement(t *testing.T) {
	t.Parallel()
	b, clk := newBank(t, "")
	c := caja(t, b, "ref-1", "ARS")
	// 23:30 in Buenos Aires on Nov 7 is 02:30 UTC on Nov 8: it belongs to the local day 7.
	clk.now = time.Date(2026, 11, 8, 2, 30, 0, 0, time.UTC)
	in, err := b.Incoming(IncomingInput{CVU: c.CVU, Amount: 1_500_00, Origin: Counterparty{Name: "Ana", CUIT: "27111111110"}})
	if err != nil || in.Type != TypeCredit || in.Status != StatusCompleted || in.Counterparty.AccountNumber == "" {
		t.Fatalf("incoming = %+v %v", in, err)
	}
	if bal, _ := b.Balance("ARS"); bal != 1_000_000_00+1_500_00 {
		t.Fatalf("balance = %d", bal)
	}
	day7 := time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)
	day8 := day7.AddDate(0, 0, 1)
	if items, total := b.ListTransfers(TransferFilter{From: day7, To: day7, Currency: "ARS"}); total != 1 || items[0].ID != in.ID {
		t.Fatalf("day 7 = %+v", items)
	}
	if _, total := b.ListTransfers(TransferFilter{From: day8, To: day8}); total != 0 {
		t.Fatal("the payin must not be on the UTC day")
	}
	if items, _ := b.ListTransfers(TransferFilter{TransferID: in.ID, Type: TypeCredit}); len(items) != 1 {
		t.Fatal("by transferenceId")
	}
	if _, err := b.Incoming(IncomingInput{CVU: AccountNumberFor("1"), Amount: 1}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown CVU err = %v", err)
	}
	if _, err := b.Incoming(IncomingInput{CVU: c.CVU, Currency: "USD", Amount: 1}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("currency mismatch err = %v", err)
	}
	if err := b.DisableCaja(c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Incoming(IncomingInput{Alias: c.Alias, Amount: 1}); !errors.Is(err, ErrCajaDisabled) {
		t.Fatalf("disabled CVU err = %v", err)
	}
	b.state.Transfers = nil
	for i := range 5 {
		b.state.Transfers = append(b.state.Transfers, &Transfer{ID: string(rune('a' + i)), Type: TypeCredit})
	}
	page, total := b.ListTransfers(TransferFilter{Start: 5, Length: 2})
	if total != 5 || len(page) != 1 || page[0].ID != "e" {
		t.Fatalf("last page = %+v (total %d)", page, total)
	}
}

// The JSON file survives a restart: the demo keeps its CVUs and balances.
func TestStatePersists(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "state", "bank.json")
	b, _ := newBank(t, file)
	c := caja(t, b, "ref-1", "ARS")
	if _, err := b.Incoming(IncomingInput{CVU: c.CVU, Amount: 42_00}); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := newBank(t, file)
	if got, err := reloaded.GetCaja(c.ID); err != nil || got.CVU != c.CVU {
		t.Fatalf("caja lost: %v", err)
	}
	if bal, _ := reloaded.Balance("ARS"); bal != 1_000_000_00+42_00 {
		t.Fatalf("balance lost: %d", bal)
	}
	reloaded.Reset()
	if len(reloaded.Snapshot().Cajas) != 0 {
		t.Fatal("reset kept cajas")
	}
}
