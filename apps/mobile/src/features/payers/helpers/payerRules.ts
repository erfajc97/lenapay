import type { Currency, Payer } from '@/shared/api/types';
import { CURRENCIES } from '@/shared/api/types';
import { classifyDestination } from '@/shared/helpers/cbu';
import { formatCuit, formatDni, isValidCuit, isValidDni } from '@/shared/helpers/cuit';

/** One CVU per payer per currency (R-CVU-1): a disabled CVU still occupies its currency. */
export function missingCurrencies(payer: Pick<Payer, 'cvus'>): Currency[] {
  return CURRENCIES.filter((currency) => !payer.cvus.some((cvu) => cvu.currency === currency));
}

/** Alias rule of the BCRA: 6–20 characters, letters, digits, dots and hyphens. */
export function aliasError(alias: string): 'validation.alias' | null {
  return classifyDestination(alias.trim()) === 'alias' ? null : 'validation.alias';
}

/** CUIT and CUIL share the same 11-digit check digit; a DNI is 7–8 digits. */
export function docError(docType: Payer['docType'], docNumber: string): 'validation.cuit' | 'validation.dni' | null {
  if (docType === 'dni') return isValidDni(docNumber) ? null : 'validation.dni';
  return isValidCuit(docNumber) ? null : 'validation.cuit';
}

export function formatDoc(docType: Payer['docType'], docNumber: string): string {
  return docType === 'dni' ? formatDni(docNumber) : formatCuit(docNumber);
}

/** What the QR encodes: the CVU itself, readable by any banking app's scanner as text. */
export function cvuQrPayload(cvu: { cvu: string }): string {
  return cvu.cvu;
}
