import { z } from 'zod';

import { docError } from '@/shared/helpers/payers';

/** Payer form (R-CVU-1). Messages are i18n keys. */
export const payerSchema = z
  .object({
    accountId: z.string().min(1, { message: 'validation.account' }),
    name: z.string().trim().min(3, { message: 'validation.payerName' }),
    docType: z.enum(['cuit', 'dni']),
    docNumber: z.string(),
    email: z.union([z.literal(''), z.email({ message: 'validation.email' })]),
    currencies: z.array(z.enum(['ARS', 'USD'])).min(1, { message: 'validation.currencies' }),
  })
  .superRefine((values, context) => {
    const error = docError(values.docType, values.docNumber);
    if (error) context.addIssue({ code: 'custom', path: ['docNumber'], message: error });
  });

export type PayerValues = z.infer<typeof payerSchema>;
