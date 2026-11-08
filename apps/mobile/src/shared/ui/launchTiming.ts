/** Animation beats of the launch screen: the isotype breathes, then the name rises in. */
export const LAUNCH_MARK_MS = 450;
export const LAUNCH_TEXT_DELAY_MS = 380;
export const LAUNCH_TEXT_MS = 380;
/** When "Lenapay by Lebane" is fully on screen. */
export const LAUNCH_TEXT_VISIBLE_AT_MS = LAUNCH_TEXT_DELAY_MS + LAUNCH_TEXT_MS;

/** The launch screen stays at least this long, so the name is read (~1.4 s fully visible). */
export const LAUNCH_MIN_MS = 2_200;
/** With Reduce Motion there is no animation: the final frame shows for this long. */
export const LAUNCH_REDUCED_MIN_MS = 1_200;
/** A ready app never waits longer than this behind the launch screen (Date.now can jump back). */
export const LAUNCH_MAX_HOLD_MS = 3_000;

/** How long a ready app still waits behind the launch screen. */
export function launchHoldMs(params: { elapsedMs: number; reduceMotion: boolean }): number {
  const minimum = params.reduceMotion ? LAUNCH_REDUCED_MIN_MS : LAUNCH_MIN_MS;
  return Math.min(LAUNCH_MAX_HOLD_MS, Math.max(0, minimum - params.elapsedMs));
}
