import { describe, expect, it } from 'vitest';

import { formatMoney, minorToInput, splitMoney } from '../format';
import { parseAmountInput } from '../parse';

// Why: the amount a person types is the amount that leaves their account. Stripping the
// comma turns `5,50` into `550` (M16), and a float split of minor units drifts a cent.
describe('parseAmountInput', () => {
  it.each([
    ['5,50', 550],
    ['5.50', 550],
    ['1.234,56', 123456],
    ['1,234.56', 123456],
    ['1.234', 123400],
    ['1.234.567,8', 123456780],
    ['150000', 15000000],
    ['0,01', 1],
    ['5,', 500],
  ])('parses %s to %i minor units', (input, minor) => {
    expect(parseAmountInput(input)).toEqual({ kind: 'ok', minor });
  });

  it('treats an empty field as no value, never 0', () => {
    expect(parseAmountInput('')).toEqual({ kind: 'empty' });
    expect(parseAmountInput('   ')).toEqual({ kind: 'empty' });
  });

  it.each(['abc', '5,5,5', '1,2345', '12.34.5', '-5', '5,505', ',5', '1.23,45'])(
    'marks %s invalid instead of correcting it',
    (input) => {
      expect(parseAmountInput(input)).toEqual({ kind: 'invalid' });
    },
  );

  it('accepts the last value under the cap and rejects the first above it', () => {
    expect(parseAmountInput('9999999999,99')).toEqual({ kind: 'ok', minor: 999_999_999_999 });
    expect(parseAmountInput('10000000000,00')).toEqual({ kind: 'invalid' });
  });
});

describe('formatMoney', () => {
  it('formats ARS and USD with es-AR grouping and comma decimals', () => {
    expect(formatMoney(150_000_00, 'ARS')).toBe('$ 150.000,00');
    expect(formatMoney(1_500_00, 'USD')).toBe('US$ 1.500,00');
    expect(formatMoney(5, 'ARS')).toBe('$ 0,05');
  });

  it('keeps the sign on debits and adds + only when asked', () => {
    expect(formatMoney(-1_234_56, 'ARS')).toBe('-$ 1.234,56');
    expect(formatMoney(1_234_56, 'ARS', { signed: true })).toBe('+$ 1.234,56');
  });

  it('splits big balances without float drift', () => {
    expect(splitMoney(9_007_199_254_740_99, 'ARS')).toEqual({ symbol: '$', units: '9.007.199.254.740', cents: ',99' });
  });

  it('round-trips the editable value through the parser', () => {
    for (const minor of [0, 1, 550, 123456, 15000000]) {
      expect(parseAmountInput(minorToInput(minor))).toEqual({ kind: 'ok', minor });
    }
  });
});
