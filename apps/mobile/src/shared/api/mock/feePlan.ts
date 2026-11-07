import type { Currency } from '../types';

/**
 * Default fee plan of the mock backend (R-FEE-1): payin %, payout fixed + %. The real plan
 * lives on the API; the mock mirrors its arithmetic so quotes on screen add up.
 */
interface FeeRule {
  fixedMinor: number;
  bps: number;
}

const PAYIN: Record<Currency, FeeRule> = {
  ARS: { fixedMinor: 0, bps: 60 },
  USD: { fixedMinor: 0, bps: 50 },
};
const PAYOUT: Record<Currency, FeeRule> = {
  ARS: { fixedMinor: 150_00, bps: 10 },
  USD: { fixedMinor: 1_00, bps: 0 },
};

/** fixed + amount × bps / 10 000, rounded half-up in integer arithmetic (no floats, MO1). */
export function applyFee(amountMinor: number, rule: FeeRule): number {
  return rule.fixedMinor + Math.floor((amountMinor * rule.bps + 5_000) / 10_000);
}

export function payinFee(amountMinor: number, currency: Currency): number {
  return applyFee(amountMinor, PAYIN[currency]);
}

export function payoutFee(amountMinor: number, currency: Currency): number {
  return applyFee(amountMinor, PAYOUT[currency]);
}
