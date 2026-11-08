import { describe, expect, it } from 'vitest';

import type { KycCase } from '@/shared/api/types';

import { kycTimeline } from '../kycTimeline';

// Why: the status screen is where a company waits to operate; a wrong step or a date on a
// step that has not happened yet reads as "approved" when it is not (R-KYC-1).
const kyc = (over: Partial<KycCase>): KycCase => ({
  id: 'k', orgId: 'o', kind: 'company', status: 'draft', documents: [], decisionReason: null, submittedAt: null, decidedAt: null, updatedAt: '2026-11-01T12:00:00Z', ...over,
});

describe('kycTimeline', () => {
  it('starts as a draft with nothing dated while the case loads', () => {
    expect(kycTimeline(undefined)).toEqual([
      { status: 'draft', state: 'current', at: null },
      { status: 'submitted', state: 'upcoming', at: null },
      { status: 'approved', state: 'upcoming', at: null },
    ]);
  });

  it('dates the submission while it is reviewed', () => {
    expect(kycTimeline(kyc({ status: 'submitted', submittedAt: '2026-11-02T10:00:00Z' }))[1]).toEqual({ status: 'submitted', state: 'current', at: '2026-11-02T10:00:00Z' });
  });

  it('marks every step done on approval and dates the decision', () => {
    const steps = kycTimeline(kyc({ status: 'approved', submittedAt: '2026-11-02T10:00:00Z', decidedAt: '2026-11-02T10:00:08Z' }));
    expect(steps.map((step) => step.state)).toEqual(['done', 'done', 'done']);
    expect(steps[2]?.at).toBe('2026-11-02T10:00:08Z');
  });

  it('ends a rejected case on the rejection', () => {
    expect(kycTimeline(kyc({ status: 'rejected', decidedAt: '2026-11-03T09:00:00Z' })).map((step) => step.status)).toEqual(['draft', 'submitted', 'rejected']);
  });
});
