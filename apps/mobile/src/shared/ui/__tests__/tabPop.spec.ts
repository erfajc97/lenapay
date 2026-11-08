import { describe, expect, it } from 'vitest';

import { TAB_POP, tabPop } from '../tabPop';

// Why: the selected tab must visibly pop (rise and grow) and the others sit flat; a wrong sign
// sinks the active icon into the home indicator, a scale below 1 makes it look disabled.
describe('tabPop', () => {
  it('rests flat at 0 and pops up and slightly larger at 1', () => {
    expect(tabPop(0)).toEqual({ translateY: 0, scale: 1 });
    expect(tabPop(1)).toEqual({ translateY: -TAB_POP.rise, scale: TAB_POP.scale });
    expect(TAB_POP.rise).toBeGreaterThanOrEqual(6);
    expect(TAB_POP.rise).toBeLessThanOrEqual(8);
    expect(TAB_POP.scale).toBeCloseTo(1.1);
  });

  it('interpolates in between (the spring passes through it)', () => {
    expect(tabPop(0.5)).toEqual({ translateY: -TAB_POP.rise / 2, scale: 1 + (TAB_POP.scale - 1) / 2 });
  });
});
