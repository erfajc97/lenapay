import type { Address, KycCase, KycDocumentKind, KycPersonData } from '@/shared/api/types';
import { digitsOnly } from '@/shared/helpers/cuit';
import { formatDayInput, parseDayInput } from '@/shared/helpers/days';

import type { CompanyValues, PersonValues } from '../types';

export interface ChecklistItem {
  key: string;
  done: boolean;
}

/** The documents that prove a person's identity (DNI both sides + selfie), as the API requires. */
export const IDENTITY_DOCS = ['dni_front', 'dni_back', 'selfie'] as const satisfies readonly KycDocumentKind[];
type IdentityDoc = (typeof IDENTITY_DOCS)[number];

function hasDocument(kyc: Pick<KycCase, 'documents'>, kind: KycDocumentKind, representativeDni: string | null = null): boolean {
  return kyc.documents.some((document) => document.kind === kind && document.representativeDni === representativeDni);
}

function hasIdentity(kyc: Pick<KycCase, 'documents'>, representativeDni: string | null): boolean {
  return IDENTITY_DOCS.every((kind) => hasDocument(kyc, kind, representativeDni));
}

/**
 * What KYC still needs before it can be submitted (R-KYC-1, R-KYC-2). Mirrors the API's
 * completeness rule (kyc `missingItems`) so "Enviar a revisión" is enabled when the API would
 * accept it; the app also asks for at least one representative of a company.
 */
export function kycChecklist(kyc: KycCase): ChecklistItem[] {
  if (kyc.kind === 'person') {
    return [
      { key: 'kyc.check.personData', done: Boolean(kyc.person) },
      { key: 'kyc.check.dniFront', done: hasDocument(kyc, 'dni_front') },
      { key: 'kyc.check.dniBack', done: hasDocument(kyc, 'dni_back') },
      { key: 'kyc.check.selfie', done: hasDocument(kyc, 'selfie') },
    ];
  }
  const representatives = kyc.company?.representatives ?? [];
  return [
    { key: 'kyc.check.companyData', done: Boolean(kyc.company) },
    { key: 'kyc.check.bylaws', done: hasDocument(kyc, 'bylaws') },
    { key: 'kyc.check.afip', done: hasDocument(kyc, 'afip_constancy') },
    { key: 'kyc.check.representatives', done: representatives.length > 0 && representatives.every((rep) => hasIdentity(kyc, rep.dni)) },
  ];
}

export function canSubmitKyc(kyc: KycCase): boolean {
  return (kyc.status === 'draft' || kyc.status === 'rejected') && kycChecklist(kyc).every((item) => item.done);
}

/** Which identity documents one person (a representative, or the person case with null) has. */
export function identityDocs(kyc: Pick<KycCase, 'documents'>, representativeDni: string | null): Record<IdentityDoc, boolean> {
  return {
    dni_front: hasDocument(kyc, 'dni_front', representativeDni),
    dni_back: hasDocument(kyc, 'dni_back', representativeDni),
    selfie: hasDocument(kyc, 'selfie', representativeDni),
  };
}

export function hasCompanyDocument(kyc: Pick<KycCase, 'documents'>, kind: 'bylaws' | 'afip_constancy'): boolean {
  return hasDocument(kyc, kind);
}

type AddressValues = Pick<CompanyValues, 'street' | 'number' | 'floor' | 'city' | 'province' | 'postalCode'>;

/** Form fields → the contract's Address (an empty floor is absent, the country is AR). */
export function toAddress(values: AddressValues): Address {
  const floor = values.floor.trim();
  return {
    street: values.street.trim(),
    number: values.number.trim(),
    ...(floor ? { floor } : {}),
    city: values.city.trim(),
    province: values.province.trim(),
    postalCode: values.postalCode.trim(),
    country: 'AR',
  };
}

function addressValues(address: Address | undefined): AddressValues {
  return {
    street: address?.street ?? '',
    number: address?.number ?? '',
    floor: address?.floor ?? '',
    city: address?.city ?? '',
    province: address?.province ?? '',
    postalCode: address?.postalCode ?? '',
  };
}

/** The saved company data (or the org name for a new case) as form values. */
export function companyFormValues(kyc: KycCase | undefined, fallbackName: string): CompanyValues {
  const company = kyc?.company;
  return { cuit: company?.cuit ?? '', legalName: company?.legalName ?? fallbackName, ...addressValues(company?.legalAddress) };
}

export function personFormValues(kyc: KycCase | undefined, fallbackName: string): PersonValues {
  const person = kyc?.person;
  return {
    fullName: person?.fullName ?? fallbackName,
    dni: person?.dni ?? '',
    birthDate: person?.birthDate ? formatDayInput(person.birthDate) : '',
    ...addressValues(person?.address),
  };
}

/** Person form → contract data (DNI digits only, birth date as a business date). */
export function toPersonData(values: PersonValues): KycPersonData {
  return { fullName: values.fullName.trim(), dni: digitsOnly(values.dni), birthDate: parseDayInput(values.birthDate) ?? '', address: toAddress(values) };
}
