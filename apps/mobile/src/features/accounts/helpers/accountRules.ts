import type { Account, Currency } from '@/shared/api/types';

type ArchiveBlocker = 'main' | 'not_empty' | 'archived' | null;

/** R-ACC-3: only project accounts, only with zero posted AND zero held in every currency. */
export function archiveBlocker(account: Pick<Account, 'kind' | 'balances' | 'archivedAt'>): ArchiveBlocker {
  if (account.archivedAt) return 'archived';
  if (account.kind !== 'project') return 'main';
  return account.balances.some((balance) => balance.posted !== 0 || balance.held !== 0) ? 'not_empty' : null;
}

export function availableIn(account: Pick<Account, 'balances'>, currency: Currency): number | null {
  return account.balances.find((balance) => balance.currency === currency)?.available ?? null;
}

/** Main first, then projects by name, archived last. */
export function sortAccounts<T extends Pick<Account, 'kind' | 'name' | 'archivedAt'>>(accounts: T[]): T[] {
  const rank = (account: T) => (account.archivedAt ? 2 : account.kind === 'project' ? 1 : 0);
  return [...accounts].sort((a, b) => rank(a) - rank(b) || a.name.localeCompare(b.name, 'es'));
}

/** Account names: trimmed, 3–40 characters. */
export function accountNameError(name: string): 'validation.accountName' | null {
  const length = name.trim().length;
  return length >= 3 && length <= 40 ? null : 'validation.accountName';
}
