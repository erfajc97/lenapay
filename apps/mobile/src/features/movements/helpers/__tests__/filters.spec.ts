import { describe, expect, it } from 'vitest';

import { activeFilterCount, toMovementFilters } from '../filters';

// Why: a filter that silently drops (or adds) a day shows the wrong money for "últimos 7
// días" when someone reconciles by hand against their bank.
describe('toMovementFilters', () => {
  const now = new Date(2026, 10, 7, 15, 30);

  it('sends nothing when no filter is set', () => {
    expect(toMovementFilters({ type: null, status: null, currency: null, range: 'all' }, now)).toEqual({});
  });

  it('starts "last 7 days" at local midnight six days ago, today included', () => {
    const filters = toMovementFilters({ type: 'payin', status: 'settled', currency: 'USD', range: '7d' }, now);
    expect(filters).toMatchObject({ type: 'payin', status: 'settled', currency: 'USD' });
    expect(new Date(filters.from ?? '').getTime()).toBe(new Date(2026, 10, 1, 0, 0, 0, 0).getTime());
  });

  it('counts active filters for the badge', () => {
    expect(activeFilterCount({ type: 'payout', status: null, currency: 'ARS', range: '30d' })).toBe(3);
    expect(activeFilterCount({ type: null, status: null, currency: null, range: 'all' })).toBe(0);
  });
});
