import type { Currency } from '@/shared/api/types';

const SYMBOLS: Record<Currency, string> = { ARS: '$', USD: 'US$' };
const groupFormatter = new Intl.NumberFormat('es-AR', { maximumFractionDigits: 0, useGrouping: true });

/**
 * `$ 1.500,00` / `US$ 1.500,00` from integer minor units (brand.md). The split into units
 * and cents is integer arithmetic: dividing by 100 as a float would drift on large amounts.
 */
export function formatMoney(minor: number, currency: Currency, options: { signed?: boolean } = {}): string {
  const negative = minor < 0;
  const abs = Math.abs(minor);
  const units = (abs - (abs % 100)) / 100;
  const cents = String(abs % 100).padStart(2, '0');
  const sign = negative ? '-' : options.signed ? '+' : '';
  return `${sign}${SYMBOLS[currency]} ${groupFormatter.format(units)},${cents}`;
}

/** The big balance splits units and cents so the cents can render smaller. */
export function splitMoney(minor: number, currency: Currency): { symbol: string; units: string; cents: string } {
  const abs = Math.abs(minor);
  const units = (abs - (abs % 100)) / 100;
  return {
    symbol: `${minor < 0 ? '-' : ''}${SYMBOLS[currency]}`,
    units: groupFormatter.format(units),
    cents: `,${String(abs % 100).padStart(2, '0')}`,
  };
}

/** Amount for an editable field: `1234,5` style, no grouping, so it parses back to the same value. */
export function minorToInput(minor: number): string {
  const units = (minor - (minor % 100)) / 100;
  const cents = minor % 100;
  return cents === 0 ? String(units) : `${units},${String(cents).padStart(2, '0')}`;
}
