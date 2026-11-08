/**
 * Geometry and motion of the floating tab bar's liquid indicator (pure worklets, so the springs
 * run on the UI thread). The pill sits behind the selected icon; when the selection moves, its
 * LEADING edge (toward the new tab) springs fast and the trailing edge follows softly, so the
 * pill stretches and then contracts like a drop of liquid.
 */
export const TAB_BAR = {
  /** Height of the floating capsule (icons + small labels). */
  height: 56,
  /** Gap above the home indicator / screen bottom: small, so the capsule sits low (owner feedback). */
  gap: 3,
  /** Side inset from the screen edges. */
  side: 16,
  /** Width of the pill behind the selected icon (narrower slots shrink it). */
  pillWidth: 52,
  pillHeight: 30,
} as const;

/** Leading edge: fast and firm. Trailing edge: softer, it arrives later. */
export const LEADING_SPRING = { stiffness: 420, damping: 34, mass: 0.7 } as const;
export const TRAILING_SPRING = { stiffness: 150, damping: 20, mass: 0.9 } as const;

/** Left and right edge (dp, inside the capsule) of the pill for a tab. */
export function pillEdges(params: { index: number; count: number; width: number }): { left: number; right: number } {
  'worklet';
  const slot = params.count > 0 ? params.width / params.count : 0;
  const pill = Math.min(TAB_BAR.pillWidth, slot);
  const left = params.index * slot + (slot - pill) / 2;
  return { left, right: left + pill };
}

/** Which edge leads a move: the right one when moving right, the left one when moving left. */
export function edgeSprings(params: { from: number; to: number }): { left: typeof LEADING_SPRING | typeof TRAILING_SPRING; right: typeof LEADING_SPRING | typeof TRAILING_SPRING } {
  'worklet';
  const movingRight = params.to > params.from;
  return { left: movingRight ? TRAILING_SPRING : LEADING_SPRING, right: movingRight ? LEADING_SPRING : TRAILING_SPRING };
}

/** The gooey squash: the more the pill stretches past its rest width, the thinner it gets (max 20 %). */
export function liquidSquash(params: { width: number; rest: number }): number {
  'worklet';
  if (params.rest <= 0 || params.width <= params.rest) return 1;
  const stretch = (params.width - params.rest) / params.rest;
  return 1 - Math.min(0.2, stretch * 0.12);
}

/** Bottom space a tab screen reserves so its last row clears the floating bar. */
export function tabBarSpace(bottomInset: number): number {
  return bottomInset + TAB_BAR.gap + TAB_BAR.height + 16;
}
