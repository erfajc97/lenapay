import { describe, expect, it } from 'vitest';

import { formatDayInput, parseDayInput } from '@/shared/helpers/days';
import { EMPTY_FILTERS, type MovementFilterDraft } from '@/shared/stores/useMovementFiltersStore';

import { activeFilterCount, customRangeError, toMovementFilters } from '../filters';

const draft = (overrides: Partial<MovementFilterDraft>): MovementFilterDraft => ({ ...EMPTY_FILTERS, ...overrides });

// Why: a filter that silently drops (or adds) a day shows the wrong money for "últimos 7
// días" when someone reconciles by hand against their bank. The API takes business dates
// (`format: date`, both ends inclusive); a timestamp is rejected by the live API with a 400.
describe('toMovementFilters', () => {
  const now = new Date(2026, 10, 7, 15, 30);

  it('sends nothing when no filter is set', () => {
    expect(toMovementFilters(EMPTY_FILTERS, now)).toEqual({});
  });

  it('starts "last 7 days" six days ago, today included, as a business date', () => {
    const filters = toMovementFilters(draft({ type: 'payin', status: 'completed', currency: 'USD', range: '7d' }), now);
    expect(filters).toEqual({ type: 'payin', status: 'completed', currency: 'USD', from: '2026-11-01' });
  });

  it('starts "last 30 days" twenty-nine days ago', () => {
    expect(toMovementFilters(draft({ range: '30d' }), now)).toEqual({ from: '2026-10-09' });
  });

  it('"today" and "this month" start today and on the 1st', () => {
    expect(toMovementFilters(draft({ range: 'today' }), now)).toEqual({ from: '2026-11-07' });
    expect(toMovementFilters(draft({ range: 'month' }), now)).toEqual({ from: '2026-11-01' });
  });

  it('pads single-digit months and days', () => {
    expect(toMovementFilters(draft({ range: 'today' }), new Date(2026, 0, 3, 0, 5))).toEqual({ from: '2026-01-03' });
  });

  it('sends both ends of a custom range', () => {
    expect(toMovementFilters(draft({ range: 'custom', customFrom: '2026-10-01', customTo: '2026-10-15' }), now)).toEqual({ from: '2026-10-01', to: '2026-10-15' });
  });
});

describe('activeFilterCount', () => {
  it('counts every active filter for the chips and the clear action', () => {
    expect(activeFilterCount(draft({ type: 'payout', currency: 'ARS', range: '30d' }))).toBe(3);
    expect(activeFilterCount(draft({ status: 'pending' }))).toBe(1);
    expect(activeFilterCount(EMPTY_FILTERS)).toBe(0);
  });
});

// Why: "Personalizado" is typed by hand; 31/02 or 2026-13-01 must never reach the API as a
// date the server shifts into March.
describe('custom day input', () => {
  it('reads DD/MM/AAAA into a business date', () => {
    expect(parseDayInput('07/11/2026')).toBe('2026-11-07');
    expect(parseDayInput(' 1/2/2026 ')).toBe('2026-02-01');
  });

  it('rejects impossible or incomplete dates', () => {
    expect(parseDayInput('31/02/2026')).toBeNull();
    expect(parseDayInput('29/02/2025')).toBeNull();
    expect(parseDayInput('00/01/2026')).toBeNull();
    expect(parseDayInput('07/13/2026')).toBeNull();
    expect(parseDayInput('07/11')).toBeNull();
    expect(parseDayInput('')).toBeNull();
  });

  it('accepts the last day of a leap February', () => {
    expect(parseDayInput('29/02/2028')).toBe('2028-02-29');
  });

  it('shows a business date back as DD/MM/AAAA', () => {
    expect(formatDayInput('2026-02-01')).toBe('01/02/2026');
  });

  it('asks for both ends, in order; a one-day range is fine', () => {
    expect(customRangeError('2026-10-01', '2026-10-15')).toBeNull();
    expect(customRangeError('2026-10-15', '2026-10-15')).toBeNull();
    expect(customRangeError('2026-10-16', '2026-10-15')).toBe('movements.filter.rangeOrder');
    expect(customRangeError(null, '2026-10-15')).toBe('movements.filter.rangeInvalid');
    expect(customRangeError('2026-10-01', null)).toBe('movements.filter.rangeInvalid');
  });
});
