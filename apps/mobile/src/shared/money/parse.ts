/**
 * Parses what a person types in an amount field into integer minor units (M16).
 * Both separators are accepted: the decimal keypad types `,` in es-AR and people paste
 * `1.234,56` or `1,234.56`. Nothing is ever stripped or corrected: anything ambiguous or out
 * of range is `invalid`, and an empty field is `empty`, never 0.
 */
type AmountParse =
  | { kind: 'empty' }
  | { kind: 'invalid' }
  | { kind: 'ok'; minor: number };

const MAX_MINOR = 999_999_999_999; // 9.999.999.999,99 — far above any payout limit.

export function parseAmountInput(raw: string): AmountParse {
  const text = raw.trim().replace(/\s/g, '');
  if (text === '') return { kind: 'empty' };
  if (!/^[0-9.,]+$/.test(text)) return { kind: 'invalid' };

  const split = splitDecimal(text);
  if (!split) return { kind: 'invalid' };
  const { integerPart, fractionPart } = split;
  if (!/^\d+$/.test(integerPart) || !/^\d{0,2}$/.test(fractionPart)) return { kind: 'invalid' };

  const minor = Number(integerPart) * 100 + Number(fractionPart.padEnd(2, '0'));
  if (!Number.isSafeInteger(minor) || minor > MAX_MINOR) return { kind: 'invalid' };
  return { kind: 'ok', minor };
}

/**
 * Decides which separator is the decimal one:
 * - both present → the LAST one is decimal, the other must group by thousands;
 * - only `,` → decimal (es-AR), a single one;
 * - only `.` → thousands when every group after it has 3 digits (`1.234`), else decimal
 *   with a single dot (`5.50`).
 */
function splitDecimal(text: string): { integerPart: string; fractionPart: string } | null {
  const lastComma = text.lastIndexOf(',');
  const lastDot = text.lastIndexOf('.');

  if (lastComma >= 0 && lastDot >= 0) {
    const decimalAt = Math.max(lastComma, lastDot);
    const groupSeparator = decimalAt === lastComma ? '.' : ',';
    const integerWithGroups = text.slice(0, decimalAt);
    if (!isGrouped(integerWithGroups, groupSeparator)) return null;
    return { integerPart: integerWithGroups.split(groupSeparator).join(''), fractionPart: text.slice(decimalAt + 1) };
  }
  if (lastComma >= 0) {
    if (text.indexOf(',') !== lastComma) return null;
    return { integerPart: text.slice(0, lastComma), fractionPart: text.slice(lastComma + 1) };
  }
  if (lastDot >= 0) {
    if (isGrouped(text, '.')) return { integerPart: text.split('.').join(''), fractionPart: '' };
    if (text.indexOf('.') !== lastDot) return null;
    return { integerPart: text.slice(0, lastDot), fractionPart: text.slice(lastDot + 1) };
  }
  return { integerPart: text, fractionPart: '' };
}

function isGrouped(text: string, separator: string): boolean {
  if (!text.includes(separator)) return /^\d+$/.test(text);
  const [head, ...groups] = text.split(separator);
  return head !== undefined && /^\d{1,3}$/.test(head) && groups.every((group) => /^\d{3}$/.test(group));
}
