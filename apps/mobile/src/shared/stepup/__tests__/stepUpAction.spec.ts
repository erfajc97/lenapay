import { describe, expect, it, vi } from 'vitest';

import { ApiError } from '@/shared/api/errors';

import { runStepUpAction } from '../stepUpAction';

// Why: ownership and the approval policy decide who moves money (R-ORG-3, R-OUT-7): the API
// refuses them without a fresh step-up, and a cancelled sheet must never reach the API.
describe('runStepUpAction', () => {
  it('sends the fresh step-up token to the call', async () => {
    const call = vi.fn(async () => 'ok');
    await expect(runStepUpAction({ call, stepUp: async () => 'su-1' })).resolves.toBe('ok');
    expect(call).toHaveBeenCalledWith({ stepUpToken: 'su-1' });
  });

  it('never calls the API when the step-up is cancelled', async () => {
    const call = vi.fn(async () => 'ok');
    const stepUp = async () => {
      throw new ApiError('stepup.cancelled', 0);
    };
    await expect(runStepUpAction({ call, stepUp })).rejects.toMatchObject({ code: 'stepup.cancelled' });
    expect(call).not.toHaveBeenCalled();
  });
});
