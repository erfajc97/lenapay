package reconciliation

import (
	"fmt"
	"slices"

	"github.com/google/uuid"

	"lenapay/apps/api/internal/platform/money"
	"lenapay/apps/api/internal/psp"
)

// match pairs the PSP statement with our ledger movements by provider id and direction. A
// line with a ledger record is matched (both amounts kept, so an amount drift shows in the
// totals); a line without one is missing_in_ledger; a record the PSP never printed is
// missing_at_psp. Output order: statement order, then unmatched ledger records.
func match(currency money.Currency, lines []psp.StatementLine, movements []LedgerMovement) []Item {
	type key struct {
		dir psp.Direction
		id  string
	}
	byKey := map[key]int{}
	for i, m := range movements {
		for _, k := range m.Keys {
			if k != "" {
				byKey[key{m.Direction, k}] = i
			}
		}
	}
	used := make([]bool, len(movements))
	items := make([]Item, 0, len(lines)+len(movements))
	for _, l := range lines {
		pspAmount := l.Amount
		it := Item{ProviderTransferID: l.TransferID, Direction: l.Direction, Currency: currency, PSPAmount: &pspAmount, Status: ItemMissingInLedger}
		if i, ok := byKey[key{l.Direction, l.TransferID}]; ok && !used[i] {
			used[i] = true
			ledgerAmount, orgID := movements[i].Amount, movements[i].OrgID
			it.LedgerAmount, it.OrgID, it.Status = &ledgerAmount, &orgID, ItemMatched
		}
		items = append(items, it)
	}
	for i, m := range movements {
		if used[i] {
			continue
		}
		ledgerAmount, orgID := m.Amount, m.OrgID
		items = append(items, Item{
			ProviderTransferID: firstKey(m.Keys), Direction: m.Direction, Currency: currency,
			LedgerAmount: &ledgerAmount, Status: ItemMissingAtPSP, OrgID: &orgID,
		})
	}
	return items
}

func firstKey(keys []string) string {
	i := slices.IndexFunc(keys, func(k string) bool { return k != "" })
	if i < 0 {
		return ""
	}
	return keys[i]
}

// forOrg keeps the items that belong to orgID (nil keeps every item: the ops run).
func forOrg(items []Item, orgID *uuid.UUID) []Item {
	if orgID == nil {
		return items
	}
	return slices.DeleteFunc(slices.Clone(items), func(it Item) bool { return it.OrgID == nil || *it.OrgID != *orgID })
}

// summarize counts the items and sums each side signed by direction (in +, out −).
func summarize(currency money.Currency, items []Item) (Totals, error) {
	t := Totals{PSP: money.Zero(currency), Ledger: money.Zero(currency)}
	for _, it := range items {
		switch it.Status {
		case ItemMatched:
			t.Matched++
		case ItemMissingInLedger:
			t.MissingInLedger++
		case ItemMissingAtPSP:
			t.MissingAtPSP++
		}
		var err error
		if t.PSP, err = addSigned(t.PSP, it.PSPAmount, it.Direction); err != nil {
			return Totals{}, err
		}
		if t.Ledger, err = addSigned(t.Ledger, it.LedgerAmount, it.Direction); err != nil {
			return Totals{}, err
		}
	}
	diff, err := t.PSP.Sub(t.Ledger)
	if err != nil {
		return Totals{}, fmt.Errorf("reconciliation: difference: %w", err)
	}
	t.Difference = diff
	return t, nil
}

func addSigned(total money.Amount, a *money.Amount, dir psp.Direction) (money.Amount, error) {
	if a == nil {
		return total, nil
	}
	if dir == psp.DirectionOut {
		return total.Sub(*a)
	}
	return total.Add(*a)
}
