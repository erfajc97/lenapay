import type { KycCase, KycDocumentType } from '@/shared/api/types';

export interface ChecklistItem {
  key: string;
  done: boolean;
}

function hasDocument(kyc: Pick<KycCase, 'documents'>, type: KycDocumentType, representativeId: string | null = null): boolean {
  return kyc.documents.some((document) => document.type === type && document.representativeId === representativeId);
}

/**
 * What KYC still needs before it can be submitted (R-KYC-1, R-KYC-2). Mirrors the API's
 * completeness rule so "Enviar a revisión" is enabled exactly when the API would accept it.
 */
export function kycChecklist(kyc: KycCase): ChecklistItem[] {
  if (kyc.kind === 'person') {
    return [
      { key: 'kyc.check.personData', done: Boolean(kyc.fullName && kyc.dni) },
      { key: 'kyc.check.dniFront', done: hasDocument(kyc, 'dni_front') },
      { key: 'kyc.check.selfie', done: hasDocument(kyc, 'selfie') },
    ];
  }
  const representativesDone =
    kyc.representatives.length > 0 && kyc.representatives.every((rep) => hasDocument(kyc, 'dni_front', rep.id) && hasDocument(kyc, 'selfie', rep.id));
  return [
    { key: 'kyc.check.companyData', done: Boolean(kyc.cuit && kyc.legalName && kyc.legalAddress) },
    { key: 'kyc.check.bylaws', done: hasDocument(kyc, 'bylaws') },
    { key: 'kyc.check.afip', done: hasDocument(kyc, 'afip_constancy') },
    { key: 'kyc.check.representatives', done: representativesDone },
  ];
}

export function canSubmitKyc(kyc: KycCase): boolean {
  return (kyc.status === 'draft' || kyc.status === 'rejected') && kycChecklist(kyc).every((item) => item.done);
}

export function representativeDocs(kyc: Pick<KycCase, 'documents'>, representativeId: string): { dniFront: boolean; selfie: boolean } {
  return { dniFront: hasDocument(kyc, 'dni_front', representativeId), selfie: hasDocument(kyc, 'selfie', representativeId) };
}

export function hasCompanyDocument(kyc: Pick<KycCase, 'documents'>, type: 'bylaws' | 'afip_constancy'): boolean {
  return hasDocument(kyc, type);
}

export function hasPersonDocument(kyc: Pick<KycCase, 'documents'>, type: 'dni_front' | 'selfie'): boolean {
  return hasDocument(kyc, type);
}
