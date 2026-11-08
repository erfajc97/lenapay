import { z } from 'zod';

/** Validation messages are i18n keys, translated by the view (M8). */
export const loginSchema = z.object({
  email: z.email({ message: 'validation.email' }),
  password: z.string().min(1, { message: 'validation.required' }),
});
export type LoginValues = z.infer<typeof loginSchema>;
