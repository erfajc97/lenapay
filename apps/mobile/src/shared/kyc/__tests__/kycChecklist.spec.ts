import { describe, expect, it } from 'vitest';

import type { KycCase, KycDocument } from '@/shared/api/types';

import { canSubmitKyc, companyFormValues, identityDocs, kycChecklist, personFormValues, toAddress, toPersonData } from '../kycChecklist';

// Why: "Enviar a revisión" enabled on an incomplete file gets a 422 from the API; disabled on
// a complete one blocks the company from ever operating (R-KYC-1, R-KYC-3).
const doc = (kind: KycDocument['kind'], representativeDni: string | null = null): KycDocument => ({
  id: `${kind}-${representativeDni}`,
  kind,
  representativeDni,
  fileName: 'f',
  contentType: 'application/pdf',
  sizeBytes: 10,
  createdAt: '2026-11-01T12:00:00Z',
});
const ADDRESS = { street: 'Av. del Libertador', number: '5800', city: 'CABA', province: 'CABA', postalCode: 'C1428', country: 'AR' };
const REP_DOCS = [doc('dni_front', '30111222'), doc('dni_back', '30111222'), doc('selfie', '30111222')];

const company = (over: Partial<KycCase> = {}): KycCase => ({
  id: 'k',
  orgId: 'o',
  kind: 'company',
  status: 'draft',
  company: { cuit: '30712345671', legalName: 'Desarrollos Del Sur SA', legalAddress: ADDRESS, representatives: [{ fullName: 'Erick Jiménez', dni: '30111222', position: 'Presidente' }] },
  documents: [doc('bylaws'), doc('afip_constancy'), ...REP_DOCS],
  decisionReason: null,
  submittedAt: null,
  decidedAt: null,
  updatedAt: '2026-11-01T12:00:00Z',
  ...over,
});

const person = (over: Partial<KycCase> = {}): KycCase =>
  company({
    kind: 'person',
    company: undefined,
    person: { fullName: 'Lucía Fernández', dni: '30111222', birthDate: '1990-05-17', address: ADDRESS },
    documents: [doc('dni_front'), doc('dni_back'), doc('selfie')],
    ...over,
  });

describe('kycChecklist', () => {
  it('lets a complete company file be submitted', () => {
    expect(kycChecklist(company()).every((item) => item.done)).toBe(true);
    expect(canSubmitKyc(company())).toBe(true);
  });

  it('blocks when a representative lacks any one identity document', () => {
    for (const missing of ['dni_front', 'dni_back', 'selfie']) {
      const documents = company().documents.filter((item) => !(item.kind === missing && item.representativeDni === '30111222'));
      expect(canSubmitKyc(company({ documents }))).toBe(false);
    }
  });

  it('blocks without company data or without a representative', () => {
    expect(canSubmitKyc(company({ company: undefined }))).toBe(false);
    const base = company().company;
    if (!base) throw new Error('fixture');
    expect(canSubmitKyc(company({ company: { ...base, representatives: [] } }))).toBe(false);
  });

  it('does not count a company-level DNI as the representative one', () => {
    expect(canSubmitKyc(company({ documents: [doc('bylaws'), doc('afip_constancy'), doc('dni_front'), doc('dni_back'), doc('selfie')] }))).toBe(false);
  });

  it('only resubmits drafts and rejected files', () => {
    expect(canSubmitKyc(company({ status: 'submitted' }))).toBe(false);
    expect(canSubmitKyc(company({ status: 'approved' }))).toBe(false);
    expect(canSubmitKyc(company({ status: 'rejected' }))).toBe(true);
  });

  it('checks the person path: data and DNI front, back and selfie', () => {
    expect(canSubmitKyc(person())).toBe(true);
    expect(canSubmitKyc(person({ person: undefined }))).toBe(false);
    expect(canSubmitKyc(person({ documents: [doc('dni_front'), doc('selfie')] }))).toBe(false);
  });

  it('reports each identity document of one representative', () => {
    expect(identityDocs(company({ documents: [doc('dni_front', '30111222')] }), '30111222')).toEqual({ dni_front: true, dni_back: false, selfie: false });
    expect(identityDocs(company(), null)).toEqual({ dni_front: false, dni_back: false, selfie: false });
  });
});

// Why: the API rejects unknown or empty address members (strict DTO); an empty floor sent as
// "" or a missing country would fail the save with a validation error the person cannot fix.
describe('KYC form mapping', () => {
  const fields = { street: ' Av. Cabildo ', number: '2150', floor: '  ', city: 'CABA', province: 'CABA', postalCode: 'C1428' };

  it('drops an empty floor and fixes the country to AR', () => {
    expect(toAddress(fields)).toEqual({ street: 'Av. Cabildo', number: '2150', city: 'CABA', province: 'CABA', postalCode: 'C1428', country: 'AR' });
    expect(toAddress({ ...fields, floor: '9B' }).floor).toBe('9B');
  });

  it('prefills a new case with the org name and a saved one with its data', () => {
    expect(companyFormValues(undefined, 'Nueva SA')).toMatchObject({ cuit: '', legalName: 'Nueva SA', street: '' });
    expect(companyFormValues(company(), 'otro')).toMatchObject({ cuit: '30712345671', legalName: 'Desarrollos Del Sur SA', street: 'Av. del Libertador', floor: '' });
    expect(personFormValues(person(), 'otro')).toMatchObject({ fullName: 'Lucía Fernández', birthDate: '17/05/1990' });
  });

  it('sends the birth date as a business date and the DNI as digits', () => {
    expect(toPersonData({ fullName: ' Lucía ', dni: '30.111.222', birthDate: '17/05/1990', ...fields })).toMatchObject({ fullName: 'Lucía', dni: '30111222', birthDate: '1990-05-17' });
  });
});
