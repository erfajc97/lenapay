import type { SignupRequest } from '@/shared/api/types';

import type { SignupValues } from '../types';

/** The signup body: email lower-cased and trimmed, the optional name trimmed and left out when blank. */
export function signupRequest(values: SignupValues): SignupRequest {
  const fullName = values.fullName.trim();
  return { email: values.email.trim().toLowerCase(), password: values.password, ...(fullName ? { fullName } : {}) };
}
