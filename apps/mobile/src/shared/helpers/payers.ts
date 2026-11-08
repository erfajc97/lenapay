import type { Currency, Payer } from '@/shared/api/types';
import { CURRENCIES } from '@/shared/api/types';
import { formatCuit, formatDni, isValidCuit, isValidDni } from '@/shared/helpers/cuit';

/** One CVU per payer per currency (R-CVU-1): a disabled CVU still occupies its currency. */
export function missingCurrencies(payer: Pick<Payer, 'cvus'>): Currency[] {
  return CURRENCIES.filter((currency) => !payer.cvus.some((cvu) => cvu.currency === currency));
}

/** CUIT and CUIL share the same 11-digit check digit; a DNI is 7–8 digits. */
export function docError(docType: Payer['docType'], docNumber: string): 'validation.cuit' | 'validation.dni' | null {
  if (docType === 'dni') return isValidDni(docNumber) ? null : 'validation.dni';
  return isValidCuit(docNumber) ? null : 'validation.cuit';
}

export function formatDoc(docType: Payer['docType'], docNumber: string): string {
  return docType === 'dni' ? formatDni(docNumber) : formatCuit(docNumber);
}
