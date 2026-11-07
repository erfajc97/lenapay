import { describe, expect, it } from 'vitest';

import { aliasError, cvuQrPayload, docError, missingCurrencies } from '../payerRules';

// Why: offering a second CVU in a currency the payer already has gets a 409 from the PSP,
// and a CVU issued against a wrong CUIT routes a buyer's money to the wrong file (R-CVU-1).
const cvu = (currency: 'ARS' | 'USD', status: 'active' | 'disabled' = 'active') => ({
  id: `c-${currency}`,
  payerId: 'p',
  currency,
  cvu: '0000031000000000000000',
  alias: 'juan.perez.torre',
  status,
});

describe('missingCurrencies', () => {
  it('lists only the currencies without a CVU, disabled ones included as taken', () => {
    expect(missingCurrencies({ cvus: [] })).toEqual(['ARS', 'USD']);
    expect(missingCurrencies({ cvus: [cvu('ARS')] })).toEqual(['USD']);
    expect(missingCurrencies({ cvus: [cvu('ARS'), cvu('USD', 'disabled')] })).toEqual([]);
  });
});

describe('payer validation', () => {
  it('validates the alias bounds', () => {
    expect(aliasError('juan.p')).toBeNull();
    expect(aliasError('juan.')).toBe('validation.alias');
    expect(aliasError('juan perez')).toBe('validation.alias');
  });

  it('validates CUIT and DNI by the selected document type', () => {
    expect(docError('CUIT', '20-28456789-8')).toBeNull();
    expect(docError('CUIT', '20-28456789-7')).toBe('validation.cuit');
    expect(docError('DNI', '28456789')).toBeNull();
    expect(docError('DNI', '284567')).toBe('validation.dni');
  });

  it('encodes the CVU in the QR', () => {
    expect(cvuQrPayload(cvu('ARS'))).toBe('0000031000000000000000');
  });
});
