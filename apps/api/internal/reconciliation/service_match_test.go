package reconciliation

import (
	"testing"

	"github.com/google/uuid"

	"lenapay/apps/api/internal/platform/money"
	"lenapay/apps/api/internal/psp"
)

func line(id string, dir psp.Direction, minor int64) psp.StatementLine {
	return psp.StatementLine{TransferID: id, Direction: dir, Amount: money.New(minor, money.ARS)}
}

func TestMatchClassifiesEveryMovement(t *testing.T) {
	t.Parallel()
	orgA, orgB := uuid.New(), uuid.New()
	lines := []psp.StatementLine{
		line("in-1", psp.DirectionIn, 50_000),   // payin we credited
		line("tr-9", psp.DirectionOut, 20_000),  // payout, printed with the PSP id
		line("po-x-1", psp.DirectionOut, 7_000), // payout, printed with our attempt reference
		line("in-2", psp.DirectionIn, 3_000),    // nobody credited it
		line("in-3", psp.DirectionOut, 1_000),   // same id as a payin but the other direction
	}
	movements := []LedgerMovement{
		{Keys: []string{"in-1"}, Direction: psp.DirectionIn, Amount: money.New(50_000, money.ARS), OrgID: orgA},
		{Keys: []string{"tr-9", "po-y-1"}, Direction: psp.DirectionOut, Amount: money.New(20_000, money.ARS), OrgID: orgA},
		{Keys: []string{"", "po-x-1"}, Direction: psp.DirectionOut, Amount: money.New(7_000, money.ARS), OrgID: orgB},
		{Keys: []string{"in-3"}, Direction: psp.DirectionIn, Amount: money.New(1_000, money.ARS), OrgID: orgB},
		{Keys: []string{"in-4"}, Direction: psp.DirectionIn, Amount: money.New(900, money.ARS), OrgID: orgA},
	}
	items := match(money.ARS, lines, movements)
	want := []struct {
		id     string
		status ItemStatus
		org    *uuid.UUID
	}{
		{"in-1", ItemMatched, &orgA},
		{"tr-9", ItemMatched, &orgA},
		{"po-x-1", ItemMatched, &orgB},
		{"in-2", ItemMissingInLedger, nil},
		{"in-3", ItemMissingInLedger, nil},
		{"in-3", ItemMissingAtPSP, &orgB},
		{"in-4", ItemMissingAtPSP, &orgA},
	}
	if len(items) != len(want) {
		t.Fatalf("items = %+v", items)
	}
	for i, w := range want {
		it := items[i]
		if it.ProviderTransferID != w.id || it.Status != w.status || (w.org == nil) != (it.OrgID == nil) || (w.org != nil && *w.org != *it.OrgID) {
			t.Errorf("item %d = %+v, want %s %s", i, it, w.id, w.status)
		}
	}

	totals, err := summarize(money.ARS, items)
	if err != nil {
		t.Fatal(err)
	}
	// PSP: +50 000 −20 000 −7 000 +3 000 −1 000 = 25 000; ledger: +50 000 −20 000 −7 000 +1 000 +900 = 24 900.
	if totals.Matched != 3 || totals.MissingInLedger != 2 || totals.MissingAtPSP != 2 ||
		totals.PSP.Minor != 25_000 || totals.Ledger.Minor != 24_900 || totals.Difference.Minor != 100 {
		t.Fatalf("totals = %+v", totals)
	}

	onlyA := forOrg(items, &orgA)
	if len(onlyA) != 3 {
		t.Fatalf("org A items = %+v", onlyA)
	}
	if len(forOrg(items, nil)) != len(items) {
		t.Fatal("the ops run keeps every item")
	}
}

func TestMatchAmountDriftShowsInTheDifference(t *testing.T) {
	t.Parallel()
	org := uuid.New()
	items := match(money.ARS, []psp.StatementLine{line("in-1", psp.DirectionIn, 10_001)},
		[]LedgerMovement{{Keys: []string{"in-1"}, Direction: psp.DirectionIn, Amount: money.New(10_000, money.ARS), OrgID: org}})
	totals, err := summarize(money.ARS, items)
	if err != nil {
		t.Fatal(err)
	}
	if totals.Matched != 1 || totals.Difference.Minor != 1 {
		t.Fatalf("totals = %+v", totals)
	}
}

func TestSummarizeEmptyDay(t *testing.T) {
	t.Parallel()
	totals, err := summarize(money.USD, nil)
	if err != nil || totals.PSP != money.Zero(money.USD) || totals.Difference != money.Zero(money.USD) || totals.Matched != 0 {
		t.Fatalf("totals = %+v, %v", totals, err)
	}
}
