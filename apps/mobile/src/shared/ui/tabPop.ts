/** How far the selected tab rises (dp) and how much it grows. */
export const TAB_POP = { rise: 7, scale: 1.1 } as const;

/**
 * Transform of a tab icon for a selection progress (0 resting → 1 selected). A worklet, so the
 * spring drives it on the UI thread (Reanimated) at 60 fps.
 */
export function tabPop(progress: number): { translateY: number; scale: number } {
  'worklet';
  return { translateY: progress === 0 ? 0 : -TAB_POP.rise * progress, scale: 1 + (TAB_POP.scale - 1) * progress };
}
