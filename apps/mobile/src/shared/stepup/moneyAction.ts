import type { MoneyActionOptions } from '@/shared/api/client';
import { idempotencyKeyFor, releaseIdempotencyKey } from '@/shared/api/idempotency';
import { requestStepUp } from '@/shared/stores/useStepUpStore';

/**
 * Every money mutation runs through here (R-ID-3, R-OUT-3): a fresh step-up first, then the
 * persisted Idempotency-Key for this scope + payload. The key is released only after the API
 * answered OK; after a timeout or failure the retry reuses it, so the money moves once.
 */
export async function runMoneyAction<T>(params: {
  scope: string;
  payload: unknown;
  call: (options: MoneyActionOptions) => Promise<T>;
  now?: () => number;
  stepUp?: () => Promise<string>;
}): Promise<T> {
  const stepUpToken = await (params.stepUp ?? requestStepUp)();
  const idempotencyKey = await idempotencyKeyFor(params.scope, params.payload, (params.now ?? Date.now)());
  const result = await params.call({ idempotencyKey, stepUpToken });
  await releaseIdempotencyKey(params.scope);
  return result;
}
