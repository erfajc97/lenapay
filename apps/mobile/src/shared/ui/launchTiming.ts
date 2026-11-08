/** The launch screen stays at least this long, so it reads as a brand moment, not a flash. */
export const LAUNCH_MIN_MS = 900;

/** How long a ready app still waits behind the launch screen (never with Reduce Motion). */
export function launchHoldMs(params: { elapsedMs: number; reduceMotion: boolean }): number {
  if (params.reduceMotion) return 0;
  return Math.max(0, LAUNCH_MIN_MS - params.elapsedMs);
}
