import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { describe, expect, it } from 'vitest';

import { type Elevation, elevation, FOCUS_RING, pressedScale, PRESSED_SCALE } from '../elevation';

// Why: shadows that are written ad hoc drift from the tokens and break dark mode (a black
// shadow on a near-black page reads as nothing, so dark surfaces lose their edge). Every
// level must carry its dark replacement, and no other file may write shadows by hand.

const LEVELS: Elevation[] = ['sm', 'md', 'lg', 'xl', 'bar', 'glow'];

describe('elevation', () => {
  it.each(LEVELS)('level %s has a light shadow token and a dark replacement', (level) => {
    const classes = elevation(level).split(' ');
    expect(classes[0]).toBe(`shadow-${level}`);
    expect(classes.filter((c) => c.startsWith('dark:shadow-'))).toHaveLength(1);
    expect(classes.some((c) => c.startsWith('active:'))).toBe(false);
  });

  it.each(LEVELS)('level %s sinks while pressed only when asked', (level) => {
    const classes = elevation(level, { pressable: true }).split(' ');
    const pressed = classes.filter((c) => c.startsWith('active:shadow-'));
    expect(pressed).toHaveLength(1);
    expect(pressed[0]).not.toBe(`active:shadow-${level}`);
  });

  it('dark surfaces get a highlight, not a shadow, except the coloured glow', () => {
    expect(elevation('md')).toContain('dark:shadow-edge');
    expect(elevation('xl')).toContain('dark:shadow-edge-strong');
    expect(elevation('glow')).toContain('dark:shadow-glow-dark');
  });

  it('the focus ring is the primary border plus the focus glow', () => {
    expect(FOCUS_RING).toBe('focus:border-primary focus:shadow-focus');
  });

  it('scales a pressed control to 0.97 and leaves a resting one alone', () => {
    expect(PRESSED_SCALE).toBe(0.97);
    expect(pressedScale({ pressed: true })).toEqual({ transform: [{ scale: 0.97 }] });
    expect(pressedScale({ pressed: false })).toBeNull();
  });
});

const SRC = fileURLToPath(new URL('../../..', import.meta.url));
const AD_HOC_SHADOW = /(^|[\s'"`:])shadow-[a-z]|shadow(Color|Offset|Opacity|Radius)\b|boxShadow|\belevation:\s*[1-9]/;

function sourceFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return name === '__tests__' ? [] : sourceFiles(path);
    return /\.tsx?$/.test(name) && !name.endsWith('.d.ts') ? [path] : [];
  });
}

describe('one elevation helper', () => {
  it('finds the source tree (the census below would pass on an empty list)', () => {
    expect(sourceFiles(SRC).length).toBeGreaterThan(50);
  });

  it('no file outside elevation.ts writes shadows by hand', () => {
    const offenders = sourceFiles(SRC)
      .filter((path) => !path.endsWith(join('shared', 'ui', 'elevation.ts')))
      .filter((path) => AD_HOC_SHADOW.test(readFileSync(path, 'utf8')));
    expect(offenders).toEqual([]);
  });

  it('the census pattern bites on the shapes it must catch', () => {
    expect(AD_HOC_SHADOW.test('className="rounded-card shadow-md"')).toBe(true);
    expect(AD_HOC_SHADOW.test('className="dark:shadow-none"')).toBe(true);
    expect(AD_HOC_SHADOW.test('{ shadowColor: c, shadowOpacity: 0.2 }')).toBe(true);
    expect(AD_HOC_SHADOW.test('{ elevation: 4 }')).toBe(true);
    expect(AD_HOC_SHADOW.test('style={{ boxShadow: x }}')).toBe(true);
    expect(AD_HOC_SHADOW.test("className={elevation('md')}")).toBe(false);
    // Turning a library's default shadow off is not a shadow.
    expect(AD_HOC_SHADOW.test('{ borderTopWidth: 0, elevation: 0 }')).toBe(false);
  });
});
