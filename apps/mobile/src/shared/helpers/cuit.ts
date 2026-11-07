/**
 * CUIT/CUIL (11 digits, AFIP mod-11 check digit) and DNI (7–8 digits) helpers. KYC and
 * payer forms validate locally so a typo never reaches onboarding or a CVU request.
 */
const WEIGHTS = [5, 4, 3, 2, 7, 6, 5, 4, 3, 2];

export function digitsOnly(value: string): string {
  return value.replace(/\D/g, '');
}

export function isValidCuit(raw: string): boolean {
  const value = digitsOnly(raw);
  if (!/^(20|23|24|27|30|33|34)\d{9}$/.test(value)) return false;
  const sum = WEIGHTS.reduce((total, weight, index) => total + weight * Number(value[index]), 0);
  const remainder = 11 - (sum % 11);
  const expected = remainder === 11 ? 0 : remainder;
  return expected !== 10 && expected === Number(value[10]);
}

export function isValidDni(raw: string): boolean {
  return /^\d{7,8}$/.test(digitsOnly(raw)) && digitsOnly(raw) === raw.replace(/\./g, '').trim();
}

/** `30712345671` → `30-71234567-1` for display. */
export function formatCuit(raw: string): string {
  const value = digitsOnly(raw);
  return value.length === 11 ? `${value.slice(0, 2)}-${value.slice(2, 10)}-${value.slice(10)}` : raw;
}

/** `28456789` → `28.456.789` for display. */
export function formatDni(raw: string): string {
  const value = digitsOnly(raw);
  return value.replace(/\B(?=(\d{3})+(?!\d))/g, '.');
}
