import type { Account, AccountKind } from '@/shared/api/types';

type ArchiveBlocker = 'main' | 'not_empty' | 'archived' | null;

/** R-ACC-3: only project accounts, only with zero posted AND zero held in every currency. */
export function archiveBlocker(account: Pick<Account, 'kind' | 'balances' | 'archived'>): ArchiveBlocker {
  if (account.archived) return 'archived';
  if (account.kind !== 'project') return 'main';
  return account.balances.some((balance) => balance.posted !== 0 || balance.held !== 0) ? 'not_empty' : null;
}

/** Main first, then projects by name, archived last. */
export function sortAccounts<T extends Pick<Account, 'kind' | 'name' | 'archived'>>(accounts: T[]): T[] {
  const rank = (account: T) => (account.archived ? 2 : account.kind === 'project' ? 1 : 0);
  return [...accounts].sort((a, b) => rank(a) - rank(b) || a.name.localeCompare(b.name, 'es'));
}

/** Account names: trimmed, 3–40 characters. */
export function accountNameError(name: string): 'validation.accountName' | null {
  const length = name.trim().length;
  return length >= 3 && length <= 40 ? null : 'validation.accountName';
}

/**
 * The names the API seeds for the accounts it creates by itself (apps/api accounts service).
 * They are English placeholders, not names a person chose, so the app labels them by kind.
 */
const API_SEEDED_NAMES: Partial<Record<AccountKind, string>> = { main: 'Main account', personal: 'Personal account' };

/** The name to show: the i18n label of a seeded main/personal account, else the name as typed. */
export function accountDisplayName(account: { kind: AccountKind; name: string }, t: (key: string) => string): string {
  return API_SEEDED_NAMES[account.kind] === account.name ? t(`accounts.defaultName.${account.kind}`) : account.name;
}
