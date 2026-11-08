import { describe, expect, it } from 'vitest';

import { signupSchema } from '../../types';
import { signupRequest } from '../signupRequest';

// Why: a blank name sent as "" is validation.failed at the API, and an untrimmed one shows with
// spaces on every screen of the team (R-ID-5).
describe('signupRequest', () => {
  it('sends the trimmed name and leaves it out when blank', () => {
    expect(signupRequest({ fullName: '  Ana Gómez ', email: ' Ana@Mail.com ', password: 'secreto123' })).toEqual({ email: 'ana@mail.com', password: 'secreto123', fullName: 'Ana Gómez' });
    expect(signupRequest({ fullName: '   ', email: 'a@b.co', password: 'secreto123' })).toEqual({ email: 'a@b.co', password: 'secreto123' });
  });

  it('accepts 120 characters and refuses 121', () => {
    const base = { email: 'a@b.co', password: 'secreto123' };
    expect(signupSchema.safeParse({ ...base, fullName: 'x'.repeat(120) }).success).toBe(true);
    expect(signupSchema.safeParse({ ...base, fullName: 'x'.repeat(121) }).success).toBe(false);
    expect(signupSchema.safeParse({ ...base, fullName: '' }).success).toBe(true);
  });
});
