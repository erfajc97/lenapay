import type { ReconciliationItem, ReconciliationRun } from '@/shared/api/types';

/** PSP statement minus ledger (R-REC-1): positive means money at the bank the ledger lacks. */
export function runDifference(run: Pick<ReconciliationRun, 'pspTotal' | 'ledgerTotal'>): number {
  return run.pspTotal - run.ledgerTotal;
}

/** A run needs attention when any item is unmatched or the totals differ. */
export function needsAttention(run: Pick<ReconciliationRun, 'pspTotal' | 'ledgerTotal' | 'missingInLedger' | 'missingAtPsp'>): boolean {
  return run.missingInLedger > 0 || run.missingAtPsp > 0 || runDifference(run) !== 0;
}

/** One item's amount, signed by direction: what the bank saw, else what the ledger recorded. */
export function itemAmount(item: Pick<ReconciliationItem, 'direction' | 'pspAmount' | 'ledgerAmount'>): number {
  const amount = item.pspAmount ?? item.ledgerAmount ?? 0;
  return item.direction === 'in' ? amount : -amount;
}
