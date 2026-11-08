import { afterEach, describe, expect, it } from 'vitest';

import { formatMoney, splitMoney } from '@/shared/money/format';

import { formatDateTime, formatDay } from '../dates';
import { activeLocale, isLanguagePreference, resolveLocale, setActiveLocale } from '../locale';

const device = (...tags: string[]) => tags.map((languageTag) => ({ languageTag, languageCode: languageTag.split('-')[0] ?? null }));

// Why: "Sistema" must land on a language we ship (an unknown device language would show raw
// keys), and switching the language must never change the money itself: still ARS or USD, only
// the separators move ($ 1.500,00 ↔ $1,500.00).
describe('resolveLocale', () => {
  it('uses the picked language as is', () => {
    expect(resolveLocale('en-US', device('es-AR'))).toBe('en-US');
    expect(resolveLocale('es-AR', device('en-US'))).toBe('es-AR');
  });

  it('follows the first device language we ship, by language when the region differs', () => {
    expect(resolveLocale('system', device('en-GB', 'es-AR'))).toBe('en-US');
    expect(resolveLocale('system', device('es-MX'))).toBe('es-AR');
    expect(resolveLocale('system', device('fr-FR', 'en-US'))).toBe('en-US');
  });

  it('falls back to es-AR when the device has nothing we ship', () => {
    expect(resolveLocale('system', device('fr-FR', 'de-DE'))).toBe('es-AR');
    expect(resolveLocale('system', [])).toBe('es-AR');
  });

  it('accepts only the options of Idioma', () => {
    expect(['system', 'es-AR', 'en-US'].every(isLanguagePreference)).toBe(true);
    expect(isLanguagePreference('pt-BR')).toBe(false);
    expect(isLanguagePreference(null)).toBe(false);
  });
});

describe('formatting follows the active locale', () => {
  afterEach(() => setActiveLocale('es-AR'));

  it('starts in es-AR', () => {
    expect(activeLocale()).toBe('es-AR');
    expect(formatMoney(123_456_789, 'ARS')).toBe('$ 1.234.567,89');
  });

  it('formats money in en-US with the same currency and the same cents', () => {
    setActiveLocale('en-US');
    expect(formatMoney(123_456_789, 'ARS')).toBe('$1,234,567.89');
    expect(formatMoney(-5, 'USD')).toBe('-US$0.05');
    expect(formatMoney(100_00, 'USD', { signed: true })).toBe('+US$100.00');
    expect(splitMoney(150_000_50, 'ARS')).toEqual({ symbol: '$', units: '150,000', cents: '.50' });
  });

  it('formats dates in the active language', () => {
    setActiveLocale('en-US');
    expect(formatDay('2026-11-05')).toBe('Thursday, November 5');
    expect(formatDateTime(new Date(2026, 10, 5, 14, 30).toISOString())).toMatch(/^Nov 5/);
    setActiveLocale('es-AR');
    expect(formatDay('2026-11-05')).toBe('Jueves, 5 de noviembre');
  });
});
