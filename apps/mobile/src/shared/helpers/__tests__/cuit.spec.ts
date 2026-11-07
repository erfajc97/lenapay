import { describe, expect, it } from 'vitest';

import { formatCuit, formatDni, isValidCuit, isValidDni } from '../cuit';

// Why: KYC and payer CVUs are issued against this number; a wrong check digit means a CVU
// tied to the wrong taxpayer, which the PSP rejects days later.
describe('CUIT', () => {
  it('accepts valid CUITs with or without dashes', () => {
    expect(isValidCuit('30-71234567-1')).toBe(true);
    expect(isValidCuit('20284567898')).toBe(true);
  });

  it('rejects a wrong check digit, a wrong prefix and a wrong length', () => {
    expect(isValidCuit('30-71234567-2')).toBe(false);
    expect(isValidCuit('99-71234567-1')).toBe(false);
    expect(isValidCuit('3071234567')).toBe(false);
  });

  it('formats for display', () => {
    expect(formatCuit('30712345671')).toBe('30-71234567-1');
  });
});

describe('DNI', () => {
  it('accepts 7 and 8 digits, with or without dots, and nothing else', () => {
    expect(isValidDni('28456789')).toBe(true);
    expect(isValidDni('28.456.789')).toBe(true);
    expect(isValidDni('1234567')).toBe(true);
    expect(isValidDni('123456')).toBe(false);
    expect(isValidDni('123456789')).toBe(false);
    expect(isValidDni('28-456-789')).toBe(false);
  });

  it('formats for display', () => {
    expect(formatDni('28456789')).toBe('28.456.789');
  });
});
