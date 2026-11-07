import { beforeEach, describe, expect, it } from 'vitest';

import { EMPTY_FILTERS, useMovementFiltersStore } from '../useMovementFiltersStore';

const store = () => useMovementFiltersStore.getState();

// Why: the hero tiles open Movements already filtered; a filter left over from before would
// show "Retenido" money mixed with other movements and the totals would not match.
describe('useMovementFiltersStore', () => {
  beforeEach(() => store().reset());

  it('showOnly replaces every filter with the given ones', () => {
    store().setFilter('type', 'payin');
    store().setFilter('range', '7d');
    store().showOnly({ status: 'pending', currency: 'USD' });
    expect(store()).toMatchObject({ ...EMPTY_FILTERS, status: 'pending', currency: 'USD' });
  });

  it('a custom range keeps both ends; a quick range drops them', () => {
    store().setCustomRange('2026-10-01', '2026-10-15');
    expect(store()).toMatchObject({ range: 'custom', customFrom: '2026-10-01', customTo: '2026-10-15' });
    store().setFilter('range', '30d');
    expect(store()).toMatchObject({ range: '30d', customFrom: null, customTo: null });
  });
});
