import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';

import { describe, expect, it } from 'vitest';

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
