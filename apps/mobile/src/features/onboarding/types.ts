import { z } from 'zod';

import { isValidCuit, isValidDni } from '@/shared/helpers/cuit';

/** KYC forms (R-KYC-1, R-KYC-2). Messages are i18n keys. */
export const companySchema = z.object({
  cuit: z.string().refine(isValidCuit, { message: 'validation.cuit' }),
  legalName: z.string().trim().min(3, { message: 'validation.legalName' }),
  legalAddress: z.string().trim().min(5, { message: 'validation.address' }),
});
export type CompanyValues = z.infer<typeof companySchema>;

export const personSchema = z.object({
  fullName: z.string().trim().min(3, { message: 'validation.fullName' }),
  dni: z.string().refine(isValidDni, { message: 'validation.dni' }),
});
export type PersonValues = z.infer<typeof personSchema>;

export const representativeSchema = personSchema;
export type RepresentativeValues = PersonValues;
