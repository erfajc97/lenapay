import type { StepUpOptions } from '@/shared/api/client';
import { requestStepUp } from '@/shared/stores/useStepUpStore';

/**
 * Owner actions that move no money but decide who does (ownership transfer, approval policy):
 * a fresh step-up first (R-ID-3), then the call with `X-Step-Up-Token`. Replacing a policy or
 * handing the org over is idempotent by nature, so no Idempotency-Key travels.
 */
export async function runStepUpAction<T>(params: { call: (options: StepUpOptions) => Promise<T>; stepUp?: () => Promise<string> }): Promise<T> {
  const stepUpToken = await (params.stepUp ?? requestStepUp)();
  return params.call({ stepUpToken });
}
