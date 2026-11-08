import type { Account, Currency } from '@/shared/api/types';
import { parseAmountInput } from '@/shared/money/parse';

/** Mover as a wizard: pick the source, then the destination, then the amount. */
export const TRANSFER_STEPS = ['from', 'to', 'amount'] as const;
export type TransferStep = (typeof TRANSFER_STEPS)[number];

/** How many accounts a step lists before "Ver todas". */
export const ACCOUNTS_PREVIEW = 3;

export function stepNumber(step: TransferStep): number {
  return TRANSFER_STEPS.indexOf(step) + 1;
}

export function nextStep(step: TransferStep): TransferStep | null {
  return TRANSFER_STEPS[TRANSFER_STEPS.indexOf(step) + 1] ?? null;
}

/** `null` on the first step: going back there leaves Mover. */
export function previousStep(step: TransferStep): TransferStep | null {
  const index = TRANSFER_STEPS.indexOf(step);
  return index > 0 ? (TRANSFER_STEPS[index - 1] ?? null) : null;
}

/** Desde needs a source; Hacia needs a destination that is not the source. */
export function canContinue(params: { step: TransferStep; fromId: string | null; toId: string | null }): boolean {
  if (params.step === 'from') return params.fromId !== null;
  if (params.step === 'to') return params.toId !== null && params.toId !== params.fromId;
  return true;
}

/** Lower case, no accents, trimmed: "Núñez" matches "nunez". */
export function searchKey(text: string): string {
  return text.normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase().trim();
}

/**
 * The accounts a step lists: the source never appears as destination; a search shows every
 * match; otherwise the first few (the selected one always among them) and how many more hide
 * behind "Ver todas".
 */
export function filterAccounts(params: { accounts: Account[]; query: string; excludeId: string | null; selectedId: string | null; expanded: boolean }): { visible: Account[]; hiddenCount: number } {
  const candidates = params.accounts.filter((account) => account.id !== params.excludeId);
  const query = searchKey(params.query);
  if (query !== '') return { visible: candidates.filter((account) => searchKey(account.name).includes(query)), hiddenCount: 0 };
  if (params.expanded) return { visible: candidates, hiddenCount: 0 };
  const selected = candidates.find((account) => account.id === params.selectedId);
  const ordered = selected ? [selected, ...candidates.filter((account) => account !== selected)] : candidates;
  return { visible: ordered.slice(0, ACCOUNTS_PREVIEW), hiddenCount: Math.max(0, ordered.length - ACCOUNTS_PREVIEW) };
}

/** "Saldo después" of the source in the chosen currency, or null when it cannot be known yet. */
export function balanceAfter(params: { from: Account | null; currency: Currency; amountText: string }): number | null {
  const parsed = parseAmountInput(params.amountText);
  if (!params.from || parsed.kind !== 'ok') return null;
  const available = params.from.balances.find((balance) => balance.currency === params.currency)?.available ?? 0;
  return available - parsed.minor;
}
