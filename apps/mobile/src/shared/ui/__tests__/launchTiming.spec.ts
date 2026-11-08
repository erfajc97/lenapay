import { describe, expect, it } from 'vitest';

import { LAUNCH_MAX_HOLD_MS, LAUNCH_MIN_MS, LAUNCH_REDUCED_MIN_MS, LAUNCH_TEXT_VISIBLE_AT_MS, launchHoldMs } from '../launchTiming';

// Why: the launch screen must stay long enough for "Lenapay by Lebane" to be read (a flash
// reads as a glitch, not a brand moment), but a ready app must never sit behind it for long:
// the hold is capped, so even a clock that jumps backwards cannot strand a cold start.
describe('launchHoldMs', () => {
  it('waits out the rest of the minimum when the app is ready early', () => {
    expect(launchHoldMs({ elapsedMs: 0, reduceMotion: false })).toBe(LAUNCH_MIN_MS);
    expect(launchHoldMs({ elapsedMs: 300, reduceMotion: false })).toBe(LAUNCH_MIN_MS - 300);
    expect(launchHoldMs({ elapsedMs: LAUNCH_MIN_MS - 1, reduceMotion: false })).toBe(1);
  });

  it('does not wait once the minimum passed', () => {
    expect(launchHoldMs({ elapsedMs: LAUNCH_MIN_MS, reduceMotion: false })).toBe(0);
    expect(launchHoldMs({ elapsedMs: 5_000, reduceMotion: false })).toBe(0);
  });

  it('shows the final frame for the shorter minimum with Reduce Motion on', () => {
    expect(launchHoldMs({ elapsedMs: 0, reduceMotion: true })).toBe(LAUNCH_REDUCED_MIN_MS);
    expect(launchHoldMs({ elapsedMs: LAUNCH_REDUCED_MIN_MS - 1, reduceMotion: true })).toBe(1);
    expect(launchHoldMs({ elapsedMs: LAUNCH_REDUCED_MIN_MS, reduceMotion: true })).toBe(0);
  });

  it('never holds a ready app longer than the cap, even with a negative elapsed time', () => {
    expect(launchHoldMs({ elapsedMs: -10_000, reduceMotion: false })).toBe(LAUNCH_MAX_HOLD_MS);
    expect(launchHoldMs({ elapsedMs: -10_000, reduceMotion: true })).toBe(LAUNCH_MAX_HOLD_MS);
  });

  it('keeps the minimums the brand moment needs', () => {
    expect(LAUNCH_MIN_MS).toBeGreaterThanOrEqual(2_100);
    expect(LAUNCH_MIN_MS).toBeLessThanOrEqual(2_400);
    // "Lenapay by Lebane" stays fully visible for at least 1.2 s before the crossfade.
    expect(LAUNCH_MIN_MS - LAUNCH_TEXT_VISIBLE_AT_MS).toBeGreaterThanOrEqual(1_200);
    expect(LAUNCH_REDUCED_MIN_MS).toBeGreaterThanOrEqual(1_100);
    expect(LAUNCH_REDUCED_MIN_MS).toBeLessThanOrEqual(1_300);
    expect(LAUNCH_MAX_HOLD_MS).toBeGreaterThanOrEqual(LAUNCH_MIN_MS);
    expect(LAUNCH_MAX_HOLD_MS).toBeLessThanOrEqual(3_000);
  });
});
