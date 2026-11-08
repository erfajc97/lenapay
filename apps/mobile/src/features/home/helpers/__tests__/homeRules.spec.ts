import { describe, expect, it } from 'vitest';

import type { Account, AccountCvu } from '@/shared/api/types';

import { balanceFor, canLinkErp, heldShare, kycActionRoute, kycNotice, personalCvus, quickActions, showErpInvite } from '../homeRules';

// Why: Home is the first screen; showing a fake $ 0 while loading reads as "the money is
// gone", and an enabled "Pagar" before KYC sends people into a dead end (M6, R-KYC-3).
describe('home rules', () => {
  const totals = [{ currency: 'ARS' as const, posted: 10, held: 2, available: 8 }];

  it('returns the currency balance or null, never a fake zero', () => {
    expect(balanceFor(totals, 'ARS')?.available).toBe(8);
    expect(balanceFor(totals, 'USD')).toBeNull();
    expect(balanceFor(undefined, 'ARS')).toBeNull();
  });

  it('opens every quick action for an approved operator', () => {
    expect(quickActions({ kycStatus: 'approved', role: 'operator' })).toEqual({ collect: null, pay: null, move: null });
  });

  it('keeps Cobrar open but blocks Pagar and Mover for a viewer', () => {
    expect(quickActions({ kycStatus: 'approved', role: 'viewer' })).toEqual({ collect: null, pay: 'role', move: 'role' });
  });

  it('blocks everything before KYC approval and says why', () => {
    expect(quickActions({ kycStatus: 'submitted', role: 'admin' })).toEqual({ collect: 'kyc', pay: 'kyc', move: 'kyc' });
    expect(kycNotice({ kycStatus: 'submitted' })).toBe('submitted');
    expect(kycNotice({ kycStatus: 'approved' })).toBeNull();
  });
});

// Why: the bar under "Retenido" is how much of the total is not spendable yet; a bar past
// 100 % or a NaN width (total 0) breaks the hero layout and lies about the money.
describe('heldShare', () => {
  const balance = (held: number, posted: number) => ({ currency: 'ARS' as const, available: posted - held, held, posted });

  it('is the held part of the total', () => {
    expect(heldShare(balance(25_00, 100_00))).toBe(0.25);
    expect(heldShare(balance(100_00, 100_00))).toBe(1);
  });

  it('is 0 with nothing held, an empty total or no balance yet', () => {
    expect(heldShare(balance(0, 100_00))).toBe(0);
    expect(heldShare(balance(0, 0))).toBe(0);
    expect(heldShare(null)).toBe(0);
  });

  it('never leaves 0..1 even if held outgrows the total', () => {
    expect(heldShare(balance(150_00, 100_00))).toBe(1);
    expect(heldShare(balance(-5_00, 100_00))).toBe(0);
  });
});

// Why: the contract has no "linked" flag on the org, so the invite reads the ERP link itself;
// inviting an org that is already linked (or a viewer who cannot link) is a dead end.
describe('ERP invite', () => {
  const admin = { kind: 'company' as const, kycStatus: 'approved' as const, role: 'admin' as const };

  it('invites an admin of a verified company that is not linked', () => {
    expect(canLinkErp(admin)).toBe(true);
    expect(canLinkErp({ ...admin, role: 'owner' })).toBe(true);
    expect(showErpInvite(admin, 'not_linked')).toBe(true);
  });

  it('hides the invite while the link is unknown, importing, linked or failed', () => {
    for (const status of [undefined, 'importing', 'linked', 'failed'] as const) expect(showErpInvite(admin, status)).toBe(false);
  });

  it('hides it for other roles, personal wallets and unverified companies', () => {
    expect(showErpInvite({ ...admin, role: 'operator' }, 'not_linked')).toBe(false);
    expect(showErpInvite({ ...admin, kind: 'personal' }, 'not_linked')).toBe(false);
    expect(showErpInvite({ ...admin, kycStatus: 'submitted' }, 'not_linked')).toBe(false);
  });
});

// Why: a person in KYC draft must land on the DNI + selfie form (the company form asks for a
// CUIT they do not have), and "Mi CVU" must never show a disabled CVU people would pay into.
describe('personal wallet home', () => {
  it('sends a person in draft to the person KYC, a company to onboarding, anyone else to the status', () => {
    expect(kycActionRoute({ kind: 'personal', kycStatus: 'draft' })).toBe('/onboarding/person');
    expect(kycActionRoute({ kind: 'company', kycStatus: 'draft' })).toBe('/onboarding');
    expect(kycActionRoute({ kind: 'personal', kycStatus: 'rejected' })).toBe('/onboarding/status');
    expect(kycActionRoute({ kind: 'personal', kycStatus: 'submitted' })).toBe('/onboarding/status');
  });

  const cvu = (id: string, currency: AccountCvu['currency'], status: AccountCvu['status'] = 'active'): AccountCvu => ({
    id, currency, cvu: id.padStart(22, '0'), alias: `${id}.alias`, status, share: { link: `https://x/${id}`, qrPayload: id },
  });
  const account = (kind: Account['kind'], cvus: AccountCvu[]): Account => ({
    id: kind, kind, name: kind, archived: false, erpProjectRef: null, balances: [], payersCount: null, cvus, createdAt: '2026-11-01T00:00:00Z',
  });

  it('lists the active CVUs of the personal account, ARS first', () => {
    const accounts = [account('main', [cvu('m', 'ARS')]), account('personal', [cvu('u', 'USD'), cvu('a', 'ARS'), cvu('d', 'ARS', 'disabled')])];
    expect(personalCvus(accounts).map((item) => item.id)).toEqual(['a', 'u']);
  });

  it('has no CVU before the person KYC or while accounts load', () => {
    expect(personalCvus([account('personal', [])])).toEqual([]);
    expect(personalCvus([account('main', [cvu('m', 'ARS')])])).toEqual([]);
    expect(personalCvus(undefined)).toEqual([]);
  });
});
