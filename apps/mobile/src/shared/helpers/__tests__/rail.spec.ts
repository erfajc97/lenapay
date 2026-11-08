import { describe, expect, it } from 'vitest';

import { RAILS } from '@/shared/api/types';

import { isInstantLenapay, RAIL_BADGE } from '../rail';

// Why: telling someone a transfer is "instantánea · sin costo" when it goes through the bank
// (fee, delay) is a broken promise about their money; the badge follows the API's rail only.
describe('rail', () => {
  it('has a badge for exactly the contract rails', () => {
    expect(Object.keys(RAIL_BADGE).sort()).toEqual([...RAILS].sort());
  });

  it('calls a transfer instant only when the API says internal', () => {
    expect(isInstantLenapay({ rail: 'internal' })).toBe(true);
    expect(isInstantLenapay({ rail: 'bank' })).toBe(false);
    expect(isInstantLenapay(null)).toBe(false);
    expect(isInstantLenapay(undefined)).toBe(false);
  });
});
