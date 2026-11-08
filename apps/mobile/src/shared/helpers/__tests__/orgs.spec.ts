import { describe, expect, it } from 'vitest';

import type { Org } from '@/shared/api/types';

import { defaultAccountId, personalOrg, pickActiveOrg, rememberedOrgId, verifiedPersonalOrgId, visibleOrgs } from '../orgs';

// Why: every money action runs against the active org; falling back to the wrong one after
// a switch, or showing a personal wallet without the B2C flag, moves money in the wrong place.
const org = (id: string, kind: Org['kind'] = 'company'): Org => ({ id, kind, legalName: id, cuit: null, kycStatus: 'approved', role: 'admin', createdAt: '2026-11-01T12:00:00Z' });

describe('org selection', () => {
  const orgs = [org('a'), org('b'), org('me', 'personal')];

  it('hides personal wallets unless personal mode is on', () => {
    expect(visibleOrgs(orgs, false).map((item) => item.id)).toEqual(['a', 'b']);
    expect(visibleOrgs(orgs, true).map((item) => item.id)).toEqual(['a', 'b', 'me']);
  });

  it('keeps the remembered org and falls back to the first visible one', () => {
    expect(pickActiveOrg(orgs, 'b')?.id).toBe('b');
    expect(pickActiveOrg(visibleOrgs(orgs, false), 'me')?.id).toBe('a');
    expect(pickActiveOrg([], 'a')).toBeNull();
  });

  it('prefers a company over a personal wallet listed first, and keeps a remembered wallet', () => {
    const walletFirst = [org('me', 'personal'), org('a')];
    expect(pickActiveOrg(walletFirst, null)?.id).toBe('a');
    expect(pickActiveOrg(walletFirst, 'me')?.id).toBe('me');
    expect(pickActiveOrg([org('me', 'personal')], null)?.id).toBe('me');
  });
});

// Why: "Mis pagos" exposes what a person paid; offering it on an unverified wallet (or a
// company membership) would only meet `kyc.not_approved`, and picking the wrong org pays from it.
describe('personal wallet', () => {
  const membership = (orgId: string, orgKind: Org['kind'], kycStatus: Org['kycStatus']) => ({ orgId, orgName: orgId, orgKind, role: 'admin' as const, kycStatus });

  it('opens Mis pagos only for an approved personal wallet', () => {
    expect(verifiedPersonalOrgId([membership('c', 'company', 'approved'), membership('p', 'personal', 'approved')])).toBe('p');
    expect(verifiedPersonalOrgId([membership('c', 'company', 'approved'), membership('p', 'personal', 'submitted')])).toBeNull();
    expect(verifiedPersonalOrgId([membership('c', 'company', 'approved')])).toBeNull();
    expect(verifiedPersonalOrgId(undefined)).toBeNull();
  });

  it('finds the personal wallet among the orgs, or none', () => {
    expect(personalOrg([org('a'), org('me', 'personal')])?.id).toBe('me');
    expect(personalOrg([org('a')])).toBeNull();
    expect(personalOrg(undefined)).toBeNull();
  });
});

// Why: the default org and account follow the person across devices (R-ID-5); a switch made
// during the session must still win, or the app jumps back to the default under their feet.
describe('rememberedOrgId', () => {
  it('prefers this session, then the synced default, then the last org on this device', () => {
    expect(rememberedOrgId({ sessionOrgId: 's', defaultOrgId: 'd', lastOrgId: 'l' })).toBe('s');
    expect(rememberedOrgId({ sessionOrgId: null, defaultOrgId: 'd', lastOrgId: 'l' })).toBe('d');
    expect(rememberedOrgId({ sessionOrgId: null, defaultOrgId: null, lastOrgId: 'l' })).toBe('l');
    expect(rememberedOrgId({ sessionOrgId: null, defaultOrgId: undefined, lastOrgId: null })).toBeNull();
  });
});

describe('defaultAccountId', () => {
  const accounts = [{ id: 'main' }, { id: 'torre' }];
  const preferences = { defaultOrgId: 'a', defaultAccountId: 'torre' };

  it('is the default account only in the default org and only while it is open', () => {
    expect(defaultAccountId(accounts, 'a', preferences)).toBe('torre');
    expect(defaultAccountId(accounts, 'b', preferences)).toBeNull();
    expect(defaultAccountId([{ id: 'main' }], 'a', preferences)).toBeNull();
    expect(defaultAccountId(accounts, 'a', { defaultOrgId: 'a', defaultAccountId: null })).toBeNull();
    expect(defaultAccountId(accounts, 'a', undefined)).toBeNull();
    expect(defaultAccountId(accounts, undefined, preferences)).toBeNull();
  });
});
