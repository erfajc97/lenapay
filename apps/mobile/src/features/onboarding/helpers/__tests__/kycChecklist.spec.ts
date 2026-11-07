import { describe, expect, it } from 'vitest';

import type { KycCase, KycDocument } from '@/shared/api/types';

import { canSubmitKyc, kycChecklist } from '../kycChecklist';

// Why: "Enviar a revisión" enabled on an incomplete file gets a 422 from the API; disabled on
// a complete one blocks the company from ever operating (R-KYC-1, R-KYC-3).
const doc = (type: KycDocument['type'], representativeId: string | null = null): KycDocument => ({ id: `${type}-${representativeId}`, type, fileName: 'f', representativeId, uploadedAt: '' });

const company = (over: Partial<KycCase> = {}): KycCase => ({
  id: 'k',
  orgId: 'o',
  kind: 'company',
  status: 'draft',
  cuit: '30712345671',
  legalName: 'Desarrollos Del Sur SA',
  legalAddress: 'Av. Libertador 5800',
  fullName: null,
  dni: null,
  representatives: [{ id: 'r1', fullName: 'Erick Jiménez', dni: '30111222' }],
  documents: [doc('bylaws'), doc('afip_constancy'), doc('dni_front', 'r1'), doc('selfie', 'r1')],
  timeline: [],
  rejectionReason: null,
  ...over,
});

describe('kycChecklist', () => {
  it('lets a complete company file be submitted', () => {
    expect(kycChecklist(company()).every((item) => item.done)).toBe(true);
    expect(canSubmitKyc(company())).toBe(true);
  });

  it('blocks when a representative lacks the selfie or there is no representative', () => {
    expect(canSubmitKyc(company({ documents: [doc('bylaws'), doc('afip_constancy'), doc('dni_front', 'r1')] }))).toBe(false);
    expect(canSubmitKyc(company({ representatives: [] }))).toBe(false);
  });

  it('does not count a company-level DNI as the representative one', () => {
    expect(canSubmitKyc(company({ documents: [doc('bylaws'), doc('afip_constancy'), doc('dni_front'), doc('selfie')] }))).toBe(false);
  });

  it('only resubmits drafts and rejected files', () => {
    expect(canSubmitKyc(company({ status: 'submitted' }))).toBe(false);
    expect(canSubmitKyc(company({ status: 'rejected' }))).toBe(true);
  });

  it('checks the person path: data, DNI front and selfie', () => {
    const person = company({ kind: 'person', fullName: 'Erick Jiménez', dni: '30111222', representatives: [], documents: [doc('dni_front')] });
    expect(canSubmitKyc(person)).toBe(false);
    expect(canSubmitKyc({ ...person, documents: [doc('dni_front'), doc('selfie')] })).toBe(true);
  });
});
