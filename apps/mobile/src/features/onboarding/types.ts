import { z } from 'zod';

import { isValidCuit, isValidDni } from '@/shared/helpers/cuit';
import { parseDayInput } from '@/shared/helpers/days';

/** KYC forms (R-KYC-1, R-KYC-2), shaped like the contract's KycCompanyData / KycPersonData. Messages are i18n keys. */
const required = (message: string) => z.string().trim().min(1, { message });

/** The contract's `Address` without `country` (always AR for now). */
const addressFields = {
  street: required('validation.street'),
  number: required('validation.streetNumber'),
  floor: z.string().trim(),
  city: required('validation.city'),
  province: required('validation.province'),
  postalCode: z.string().trim().min(4, { message: 'validation.postalCode' }),
};

export const companySchema = z.object({
  cuit: z.string().refine(isValidCuit, { message: 'validation.cuit' }),
  legalName: z.string().trim().min(3, { message: 'validation.legalName' }),
  ...addressFields,
});
export type CompanyValues = z.infer<typeof companySchema>;

export const personSchema = z.object({
  fullName: z.string().trim().min(3, { message: 'validation.fullName' }),
  dni: z.string().refine(isValidDni, { message: 'validation.dni' }),
  birthDate: z.string().refine((text) => parseDayInput(text) !== null, { message: 'validation.birthDate' }),
  ...addressFields,
});
export type PersonValues = z.infer<typeof personSchema>;

export const representativeSchema = z.object({
  fullName: z.string().trim().min(3, { message: 'validation.fullName' }),
  dni: z.string().refine(isValidDni, { message: 'validation.dni' }),
  position: z.string().trim().min(2, { message: 'validation.position' }),
});
export type RepresentativeValues = z.infer<typeof representativeSchema>;
