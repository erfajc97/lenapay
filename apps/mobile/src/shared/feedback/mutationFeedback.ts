import { ApiError, errorMessageKey } from '@/shared/api/errors';
import { i18n } from '@/shared/i18n';
import { showToast } from '@/shared/stores/useToastStore';

/**
 * The feedback mutations emit in onSuccess / onError (M2). Errors map their code to copy;
 * a step-up the person closed is not an error worth a toast.
 */
export function notifyError(error: unknown): void {
  if (error instanceof ApiError && error.code === 'stepup.cancelled') return;
  showToast('error', i18n.t(errorMessageKey(error)));
}

export function notifySuccess(key: string, params?: Record<string, string>): void {
  showToast('success', i18n.t(key, params));
}
