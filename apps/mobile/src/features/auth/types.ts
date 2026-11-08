import { z } from 'zod';

/** Validation messages are i18n keys, translated by the view (M8). */
export const loginSchema = z.object({
  email: z.email({ message: 'validation.email' }),
  password: z.string().min(1, { message: 'validation.required' }),
});
export type LoginValues = z.infer<typeof loginSchema>;

export const signupSchema = z.object({
  /** Optional (R-ID-5): blank is "no name", sent as absent; the contract caps it at 120. */
  fullName: z.string().refine((value) => value.trim().length <= 120, { message: 'validation.fullNameLength' }),
  email: z.email({ message: 'validation.email' }),
  password: z
    .string()
    .min(8, { message: 'validation.passwordLength' })
    .regex(/\d/, { message: 'validation.passwordDigit' }),
});
export type SignupValues = z.infer<typeof signupSchema>;

export const verifySchema = z.object({
  code: z.string().regex(/^\d{6}$/, { message: 'validation.code' }),
});
export type VerifyValues = z.infer<typeof verifySchema>;
