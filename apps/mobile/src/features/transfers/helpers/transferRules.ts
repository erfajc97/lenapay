import type { Account, Currency } from '@/shared/api/types';
import { parseAmountInput } from '@/shared/money/parse';

type TransferBlocker = 'from' | 'to' | 'same_account' | 'amount' | 'insufficient' | null;

/**
 * Internal transfer (R-ACC-4): two different open accounts, same currency, a positive amount
 * that fits in the source's available balance. Returns the first reason it cannot go.
 */
export function transferBlocker(input: { from: Account | null; to: Account | null; currency: Currency; amountText: string }): TransferBlocker {
  if (!input.from) return 'from';
  if (!input.to) return 'to';
  if (input.from.id === input.to.id) return 'same_account';
  const parsed = parseAmountInput(input.amountText);
  if (parsed.kind !== 'ok' || parsed.minor <= 0) return 'amount';
  const available = input.from.balances.find((balance) => balance.currency === input.currency)?.available ?? 0;
  return parsed.minor > available ? 'insufficient' : null;
}
