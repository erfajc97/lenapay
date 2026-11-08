import { z } from 'zod';

/** Validation messages are i18n keys, translated by the view (M8). */
export const verifySchema = z.object({
  code: z.string().regex(/^\d{6}$/, { message: 'validation.code' }),
});
export type VerifyValues = z.infer<typeof verifySchema>;
