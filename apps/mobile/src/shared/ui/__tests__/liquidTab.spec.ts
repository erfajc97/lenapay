import { describe, expect, it } from 'vitest';

import { edgeSprings, LEADING_SPRING, liquidSquash, pillEdges, TAB_BAR, tabBarSpace, TRAILING_SPRING } from '../liquidTab';

// Why: the pill must land exactly behind the selected icon (an offset reads as the wrong tab
// selected), lead with the edge facing the new tab (the liquid look), and screens must keep
// their last row above the floating bar or it is unreachable.
describe('liquid tab indicator', () => {
  it('centres the pill in each slot and keeps it inside the capsule', () => {
    expect(pillEdges({ index: 0, count: 5, width: 350 })).toEqual({ left: 9, right: 61 });
    expect(pillEdges({ index: 4, count: 5, width: 350 })).toEqual({ left: 289, right: 341 });
  });

  it('shrinks the pill to the slot when slots are narrower than the pill', () => {
    expect(pillEdges({ index: 1, count: 5, width: 200 })).toEqual({ left: 40, right: 80 });
    expect(pillEdges({ index: 0, count: 0, width: 200 })).toEqual({ left: 0, right: 0 });
  });

  it('leads with the edge that faces the new tab', () => {
    expect(edgeSprings({ from: 0, to: 3 })).toEqual({ left: TRAILING_SPRING, right: LEADING_SPRING });
    expect(edgeSprings({ from: 3, to: 1 })).toEqual({ left: LEADING_SPRING, right: TRAILING_SPRING });
    expect(LEADING_SPRING.stiffness).toBeGreaterThan(TRAILING_SPRING.stiffness);
  });

  it('squashes only while stretched, never below 80 %', () => {
    expect(liquidSquash({ width: 52, rest: 52 })).toBe(1);
    expect(liquidSquash({ width: 40, rest: 52 })).toBe(1);
    expect(liquidSquash({ width: 104, rest: 52 })).toBeCloseTo(0.88);
    expect(liquidSquash({ width: 400, rest: 52 })).toBe(0.8);
    expect(liquidSquash({ width: 10, rest: 0 })).toBe(1);
  });

  it('reserves the bar, its gap and the home indicator at the bottom of tab screens', () => {
    expect(tabBarSpace(34)).toBe(34 + TAB_BAR.gap + TAB_BAR.height + 16);
    expect(tabBarSpace(0)).toBe(TAB_BAR.gap + TAB_BAR.height + 16);
  });
});
