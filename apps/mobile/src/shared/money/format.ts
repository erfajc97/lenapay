import type { Currency } from '@/shared/api/types';
import { activeLocale, type AppLocale } from '@/shared/i18n/locale';

const SYMBOLS: Record<Currency, string> = { ARS: '$', USD: 'US$' };

/** Per language: thousands grouping, decimal mark and whether the symbol is spaced (brand.md). */
const STYLE: Record<AppLocale, { decimal: string; symbolGap: string }> = {
  'es-AR': { decimal: ',', symbolGap: ' ' },
  'en-US': { decimal: '.', symbolGap: '' },
};
const groupers = new Map<AppLocale, Intl.NumberFormat>();

function groupUnits(units: number): string {
  const locale = activeLocale();
  let grouper = groupers.get(locale);
  if (!grouper) {
    grouper = new Intl.NumberFormat(locale, { maximumFractionDigits: 0, useGrouping: true });
    groupers.set(locale, grouper);
  }
  return grouper.format(units);
}

/**
 * `$ 1.500,00` (es-AR) / `$1,500.00` (en-US) from integer minor units; the currency never
 * changes with the language. The split into units and cents is integer arithmetic: dividing by
 * 100 as a float would drift on large amounts.
 */
export function formatMoney(minor: number, currency: Currency, options: { signed?: boolean } = {}): string {
  const parts = splitMoney(minor, currency);
  const sign = minor >= 0 && options.signed ? '+' : '';
  return `${sign}${parts.symbol}${STYLE[activeLocale()].symbolGap}${parts.units}${parts.cents}`;
}

/** The big balance splits units and cents so the cents can render smaller. */
export function splitMoney(minor: number, currency: Currency): { symbol: string; units: string; cents: string } {
  const abs = Math.abs(minor);
  const units = (abs - (abs % 100)) / 100;
  return {
    symbol: `${minor < 0 ? '-' : ''}${SYMBOLS[currency]}`,
    units: groupUnits(units),
    cents: `${STYLE[activeLocale()].decimal}${String(abs % 100).padStart(2, '0')}`,
  };
}

/** Amount for an editable field: `1234,5` style, no grouping, so it parses back to the same value. */
export function minorToInput(minor: number): string {
  const units = (minor - (minor % 100)) / 100;
  const cents = minor % 100;
  return cents === 0 ? String(units) : `${units},${String(cents).padStart(2, '0')}`;
}
