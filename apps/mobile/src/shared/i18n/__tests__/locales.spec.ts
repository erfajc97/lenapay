import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';

import { describe, expect, it } from 'vitest';

import { APP_LOCALES, LANGUAGE_LABEL_KEY, SOURCE_LOCALE } from '../locale';
import esAR from '../locales/es-AR.json';

// Why: a missing key shows "payout.confirm" to the person about to move money (M8). This
// scans the source for every literal key and every dynamic key family and checks es-AR.
const SRC = join(__dirname, '..', '..', '..');

function sourceFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return name === '__tests__' ? [] : sourceFiles(path);
    return /\.(ts|tsx)$/.test(name) ? [path] : [];
  });
}

function resolve(key: string): unknown {
  return key.split('.').reduce<unknown>((node, part) => (node && typeof node === 'object' ? (node as Record<string, unknown>)[part] : undefined), esAR);
}

function exists(key: string): boolean {
  return typeof resolve(key) === 'string' || (typeof resolve(`${key}_one`) === 'string' && typeof resolve(`${key}_other`) === 'string');
}

const LITERAL = /(?:\bt|i18n\.t|notifySuccess)\(\s*'([a-zA-Z][\w.]*)'/g;
const KEY_CONSTANT = /'((?:validation|kyc\.check|errors)\.[a-zA-Z_.]+)'/g;

/** Keys built at runtime: prefix + every value the code can produce. */
const DYNAMIC: Record<string, readonly string[]> = {
  'status.payout': ['pending_approval', 'approved', 'sending', 'settled', 'failed', 'rejected', 'cancelled'],
  'status.movement': ['pending', 'settled', 'failed', 'cancelled'],
  'status.kyc': ['draft', 'submitted', 'approved', 'rejected'],
  roles: ['viewer', 'operator', 'approver', 'admin'],
  roleHelp: ['viewer', 'operator', 'approver', 'admin'],
  currency: ['ARS', 'USD'],
  blockers: ['kyc', 'role'],
  'accounts.kind': ['main', 'project', 'personal'],
  'accounts.archiveBlocked': ['not_empty', 'main', 'archived'],
  'approvals.blocked': ['creator', 'role'],
  'kyc.timeline': ['draft', 'submitted', 'approved', 'rejected'],
  'kyc.doc': ['bylaws', 'afip_constancy', 'dni_front', 'selfie'],
  'movements.range': ['today', '7d', '30d', 'month', 'custom'],
  'movements.filter': ['type', 'status', 'currency', 'date'],
  'settings.theme': ['system', 'light', 'dark'],
  'settings.languages': ['system', 'esAR', 'enUS'],
  'movements.type': ['payin', 'payout', 'transfer'],
  'movements.kind': ['payin', 'payout', 'transfer'],
  'movements.title': ['payin.in', 'payin.out', 'payout.in', 'payout.out', 'transfer.in', 'transfer.out'],
  'payers.cvuStatus': ['active', 'disabled'],
  'payout.destinationHint': ['incomplete', 'invalid', 'cbu', 'cvu', 'alias'],
  'payout.destinationType': ['cbu', 'cvu', 'alias'],
  'payout.result': ['pending_approval.title', 'pending_approval.body', 'sent.title', 'sent.body'],
  'reconciliation.item': ['matched', 'missing_in_ledger', 'missing_at_psp'],
  'transfer.blocker': ['from', 'to', 'same_account', 'amount', 'insufficient'],
  'home.kyc': ['draft.title', 'draft.body', 'draft.action', 'submitted.title', 'submitted.body', 'submitted.action', 'rejected.title', 'rejected.body', 'rejected.action'],
  'security.push': ['enabled', 'denied', 'unavailable'],
};

const TERNARY_KEYS = ['accounts.archived', 'accounts.renamed', 'approvals.approved', 'approvals.rejected', 'security.biometricsOn', 'security.biometricsOff'];

describe('es-AR locale', () => {
  const files = sourceFiles(SRC);
  const literal = new Set<string>();
  for (const file of files) {
    const text = readFileSync(file, 'utf8');
    for (const match of text.matchAll(LITERAL)) if (match[1]) literal.add(match[1]);
    for (const match of text.matchAll(KEY_CONSTANT)) if (match[1] && !file.includes(join('api', 'mock'))) literal.add(match[1]);
  }

  it('has every literal key used in the source', () => {
    const missing = [...literal].filter((key) => key.includes('.') && !exists(key));
    expect(missing).toEqual([]);
  });

  it('has every dynamic key family and ternary key', () => {
    const missing = [
      ...Object.entries(DYNAMIC).flatMap(([prefix, values]) => values.map((value) => `${prefix}.${value}`)),
      ...TERNARY_KEYS,
    ].filter((key) => !exists(key));
    expect(missing).toEqual([]);
  });

  it('has copy for every known API error code', async () => {
    const { errorMessageKey, ApiError } = await import('@/shared/api/errors');
    const codes = readFileSync(join(SRC, 'shared', 'api', 'errors.ts'), 'utf8').match(/'[a-z]+\.[a-z_]+'/g) ?? [];
    const missing = codes
      .map((quoted) => quoted.slice(1, -1))
      .map((code) => errorMessageKey(new ApiError(code, 400)))
      .filter((key) => !exists(key));
    expect(missing).toEqual([]);
  });
});

/** Every leaf of a catalog as `dotted.key → text`. */
function leaves(node: unknown, prefix = ''): Map<string, string> {
  const out = new Map<string, string>();
  for (const [key, value] of Object.entries(node as Record<string, unknown>)) {
    const path = prefix ? `${prefix}.${key}` : key;
    if (typeof value === 'string') out.set(path, value);
    else for (const [inner, text] of leaves(value, path)) out.set(inner, text);
  }
  return out;
}

const placeholders = (text: string) => [...text.matchAll(/\{\{(\w+)\}\}/g)].map((match) => match[1]).sort();

// Why: a language that misses a key shows the raw key ("payout.confirm") or, worse, falls back
// to Spanish in the middle of an English screen; a missing {{amount}} hides how much money moves.
describe('every shipped locale', () => {
  const dir = join(__dirname, '..', 'locales');
  const source = leaves(esAR);

  it('ships one catalog per app locale, and nothing else', () => {
    expect(readdirSync(dir).sort()).toEqual(APP_LOCALES.map((locale) => `${locale}.json`).sort());
    expect(APP_LOCALES).toContain(SOURCE_LOCALE);
  });

  it.each(APP_LOCALES.filter((locale) => locale !== SOURCE_LOCALE))('%s has exactly the keys of es-AR, with the same placeholders', (locale) => {
    const catalog = leaves(JSON.parse(readFileSync(join(dir, `${locale}.json`), 'utf8')));
    expect([...catalog.keys()].filter((key) => !source.has(key))).toEqual([]);
    expect([...source.keys()].filter((key) => !catalog.has(key))).toEqual([]);
    const mismatched = [...source].filter(([key, text]) => JSON.stringify(placeholders(text)) !== JSON.stringify(placeholders(catalog.get(key) ?? '')));
    expect(mismatched.map(([key]) => key)).toEqual([]);
    expect([...catalog.values()].filter((text) => text.trim() === '')).toEqual([]);
  });

  it('names every option of Idioma', () => {
    expect(Object.values(LANGUAGE_LABEL_KEY).filter((key) => !exists(key))).toEqual([]);
  });
});
