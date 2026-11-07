package ledger

import (
	"errors"
	"math"
	"testing"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
)

// validateEntry is the code-side barrier before any row is locked; the database repeats the
// balance rule at commit (integration tests).
func TestValidateEntry(t *testing.T) {
	t.Parallel()
	a, b, c := ids.New(), ids.New(), ids.New()
	entry := func(legs ...Leg) Entry {
		return Entry{Kind: KindAdjustment, RefType: "test", RefID: ids.New(), Legs: legs}
	}
	leg := func(account [16]byte, minor int64, cur money.Currency) Leg {
		return Leg{AccountID: account, Amount: money.New(minor, cur)}
	}

	t.Run("nets legs per account and currency", func(t *testing.T) {
		t.Parallel()
		deltas, err := validateEntry(entry(leg(a, -100, money.ARS), leg(b, 60, money.ARS), leg(b, 40, money.ARS), leg(c, -5, money.USD), leg(a, 5, money.USD)))
		if err != nil {
			t.Fatal(err)
		}
		want := map[legKey]int64{{a, money.ARS}: -100, {b, money.ARS}: 100, {c, money.USD}: -5, {a, money.USD}: 5}
		if len(deltas) != len(want) {
			t.Fatalf("deltas = %v, want %v", deltas, want)
		}
		for k, v := range want {
			if deltas[k] != v {
				t.Fatalf("delta %v = %d, want %d", k, deltas[k], v)
			}
		}
	})
	t.Run("int64 overflow is an error, never a wrap", func(t *testing.T) {
		t.Parallel()
		_, err := validateEntry(entry(leg(a, math.MaxInt64, money.ARS), leg(a, 1, money.ARS), leg(b, math.MinInt64+1, money.ARS)))
		if !errors.Is(err, money.ErrOverflow) {
			t.Fatalf("err = %v, want money.ErrOverflow", err)
		}
	})
	t.Run("ref and kind are required", func(t *testing.T) {
		t.Parallel()
		for name, e := range map[string]Entry{
			"no ref id":         {Kind: KindPayin, RefType: "payin", Legs: entry(leg(a, -1, money.ARS), leg(b, 1, money.ARS)).Legs},
			"no ref type":       {Kind: KindPayin, RefID: ids.New(), Legs: entry(leg(a, -1, money.ARS), leg(b, 1, money.ARS)).Legs},
			"ref type too long": {Kind: KindPayin, RefType: string(make([]byte, maxRefTypeLen+1)), RefID: ids.New(), Legs: entry(leg(a, -1, money.ARS), leg(b, 1, money.ARS)).Legs},
		} {
			if _, err := validateEntry(e); !errors.Is(err, ErrInvalidEntry) {
				t.Fatalf("%s: err = %v, want ErrInvalidEntry", name, err)
			}
		}
	})
	t.Run("value never crosses currencies", func(t *testing.T) {
		t.Parallel()
		_, err := validateEntry(entry(leg(a, -100, money.ARS), leg(b, 100, money.USD)))
		if !errors.Is(err, ErrUnbalanced) || !apperr.IsCode(err, apperr.CodeCurrencyMismatch) {
			t.Fatalf("err = %v, want ErrUnbalanced with money.currency_mismatch", err)
		}
	})
}
