import { describe, expect, it, vi } from 'vitest';

import { ApiError } from '@/shared/api/errors';

import { runMoneyAction } from '../moneyAction';

// Why: a retry after a timeout must reuse the Idempotency-Key or the payout leaves twice; a
// cancelled step-up must never reach the API (R-ID-3, R-OUT-3, MO2).
describe('runMoneyAction', () => {
  const payload = { amount: 100_00, to: 'hormigon.plata' };

  it('sends the step-up token and an idempotency key', async () => {
    const call = vi.fn(async () => 'ok');
    await runMoneyAction({ scope: 'payout:1', payload, call, stepUp: async () => 'su-1' });
    expect(call).toHaveBeenCalledWith({ stepUpToken: 'su-1', idempotencyKey: expect.any(String) });
  });

  it('reuses the key after a failure and issues a new one after success', async () => {
    const keys: string[] = [];
    const failing = vi.fn(async (options: { idempotencyKey: string }) => {
      keys.push(options.idempotencyKey);
      throw new ApiError('network.timeout', 0);
    });
    await expect(runMoneyAction({ scope: 'payout:2', payload, call: failing, stepUp: async () => 'su' })).rejects.toMatchObject({ code: 'network.timeout' });
    const succeeding = vi.fn(async (options: { idempotencyKey: string }) => {
      keys.push(options.idempotencyKey);
      return 'ok';
    });
    await runMoneyAction({ scope: 'payout:2', payload, call: succeeding, stepUp: async () => 'su' });
    await runMoneyAction({ scope: 'payout:2', payload, call: succeeding, stepUp: async () => 'su' });
    expect(keys[0]).toBe(keys[1]);
    expect(keys[2]).not.toBe(keys[1]);
  });

  it('never calls the API when the step-up is cancelled', async () => {
    const call = vi.fn(async () => 'ok');
    const stepUp = async () => {
      throw new ApiError('stepup.cancelled', 0);
    };
    await expect(runMoneyAction({ scope: 'payout:3', payload, call, stepUp })).rejects.toMatchObject({ code: 'stepup.cancelled' });
    expect(call).not.toHaveBeenCalled();
  });
});
