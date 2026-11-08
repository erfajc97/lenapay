import { describe, expect, it } from 'vitest';

import { LAUNCH_MIN_MS, launchHoldMs } from '../launchTiming';

// Why: the launch screen must be seen long enough to read as a brand moment (not a flash), but
// must never hold a ready app back once that minimum passed; with Reduce Motion there is no
// animation to wait for, so the app shows as soon as it is ready.
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

  it('never waits with Reduce Motion on', () => {
    expect(launchHoldMs({ elapsedMs: 0, reduceMotion: true })).toBe(0);
  });

  it('keeps the minimum around 900 ms', () => {
    expect(LAUNCH_MIN_MS).toBeGreaterThanOrEqual(800);
    expect(LAUNCH_MIN_MS).toBeLessThanOrEqual(1_000);
  });
});
