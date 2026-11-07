/**
 * The ONE elevation helper (docs/brand.md "Elevation"). Shadows are Tailwind classes backed by
 * the `--shadow-*` tokens of global.css: uniwind turns them into the `boxShadow` style, which
 * React Native draws on iOS and Android (new architecture) and react-native-web on the web.
 * Every level pairs its light shadow with a dark replacement, because a shadow does not read
 * on a near-black page: dark surfaces get a top highlight instead. Features never write
 * shadow classes or shadow styles themselves; they pick a level here (or use a primitive that
 * already does).
 */
export type Elevation = 'sm' | 'md' | 'lg' | 'xl' | 'bar' | 'glow';

const RESTING: Record<Elevation, string> = {
  sm: 'shadow-sm dark:shadow-edge',
  md: 'shadow-md dark:shadow-edge',
  lg: 'shadow-lg dark:shadow-edge-strong',
  xl: 'shadow-xl dark:shadow-edge-strong',
  bar: 'shadow-bar dark:shadow-edge-strong',
  glow: 'shadow-glow dark:shadow-glow-dark',
};

/** While pressed the surface sinks one level, together with the scale of `pressedScale`. */
const PRESSED: Record<Elevation, string> = {
  sm: 'active:shadow-none',
  md: 'active:shadow-sm',
  lg: 'active:shadow-md',
  xl: 'active:shadow-lg',
  bar: 'active:shadow-sm',
  glow: 'active:shadow-glow-pressed',
};

/** Shadow classes for a surface; `pressable` adds the pressed (sunken) state for Pressables. */
export function elevation(level: Elevation, options: { pressable?: boolean } = {}): string {
  return options.pressable ? `${RESTING[level]} ${PRESSED[level]}` : RESTING[level];
}

/** Focus ring of inputs: primary border plus a soft primary glow. */
export const FOCUS_RING = 'focus:border-primary focus:shadow-focus';

export const PRESSED_SCALE = 0.97;

/** Pressable `style` callback: pressed controls shrink slightly so the tap is felt. */
export function pressedScale({ pressed }: { pressed: boolean }) {
  return pressed ? { transform: [{ scale: PRESSED_SCALE }] } : null;
}
