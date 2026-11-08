import { describe, expect, it } from 'vitest';

import { docError, formatDoc, missingCurrencies } from '../payers';

// Why: offering a second CVU in a currency the payer already has gets a 409 from the PSP,
// and a CVU issued against a wrong CUIT routes a buyer's money to the wrong file (R-CVU-1).
const cvu = (currency: 'ARS' | 'USD', status: 'active' | 'disabled' = 'active') => ({
  id: `c-${currency}`,
  payerId: 'p',
  accountId: 'a',
  currency,
  cvu: '0000031000000000000000',
  alias: 'juan.perez.torre',
  status,
  share: { link: 'https://pay.lenapay.test/0000031000000000000000', qrPayload: 'https://pay.lenapay.test/0000031000000000000000?currency=ARS' },
  createdAt: '2026-11-01T12:00:00Z',
});

describe('missingCurrencies', () => {
  it('lists only the currencies without a CVU, disabled ones included as taken', () => {
    expect(missingCurrencies({ cvus: [] })).toEqual(['ARS', 'USD']);
    expect(missingCurrencies({ cvus: [cvu('ARS')] })).toEqual(['USD']);
    expect(missingCurrencies({ cvus: [cvu('ARS'), cvu('USD', 'disabled')] })).toEqual([]);
  });
});

describe('payer validation', () => {
  it('validates CUIT and DNI by the selected document type', () => {
    expect(docError('cuit', '20-28456789-8')).toBeNull();
    expect(docError('cuit', '20-28456789-7')).toBe('validation.cuit');
    expect(docError('cuil', '20-28456789-8')).toBeNull();
    expect(docError('cuil', '20-28456789-7')).toBe('validation.cuit');
    expect(docError('dni', '28456789')).toBeNull();
    expect(docError('dni', '284567')).toBe('validation.dni');
  });

  it('formats a DNI with dots and a CUIT or CUIL with dashes', () => {
    expect(formatDoc('dni', '28456789')).toBe('28.456.789');
    expect(formatDoc('cuit', '20284567898')).toBe('20-28456789-8');
    expect(formatDoc('cuil', '20284567898')).toBe('20-28456789-8');
  });
});
