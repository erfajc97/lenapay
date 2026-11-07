import { describe, expect, it } from 'vitest';

import type { Org } from '@/shared/api/types';

import { pickActiveOrg, visibleOrgs } from '../orgs';

// Why: every money action runs against the active org; falling back to the wrong one after
// a switch, or showing a personal wallet without the B2C flag, moves money in the wrong place.
const org = (id: string, kind: Org['kind'] = 'company'): Org => ({ id, kind, legalName: id, cuit: null, kycStatus: 'approved', role: 'admin', erpLinked: false });

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
});
