import { describe, expect, it } from 'vitest';

import { findCachedMovement } from '../findCachedMovement';

// Why: the detail screen opens from any filtered list; it must find the row in whichever
// cached page holds it, and say "not found" instead of showing another movement.
describe('findCachedMovement', () => {
  const movement = (id: string) => ({ id }) as never;
  const data = (ids: string[][]) => ({ pages: ids.map((items) => ({ items: items.map(movement), nextCursor: null })), pageParams: [] });

  it('finds the movement in any list and page', () => {
    expect(findCachedMovement([data([['a']]), data([['b'], ['c']])], 'c')).toEqual({ id: 'c' });
  });

  it('returns null when no loaded list has it', () => {
    expect(findCachedMovement([undefined, data([['a']])], 'z')).toBeNull();
  });
});
