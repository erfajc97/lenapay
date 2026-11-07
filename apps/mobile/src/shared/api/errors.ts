import type { ApiErrorBody } from './types';

/**
 * Every failure the app shows comes through this class: the `code` is the contract's error
 * code (`payout.insufficient_funds`), mapped to i18n copy by `errorMessageKey`. The server
 * message is never shown to the user (M2).
 */
export class ApiError extends Error {
  readonly code: string;
  readonly status: number;

  constructor(code: string, status: number, message?: string) {
    super(message ?? code);
    this.name = 'ApiError';
    this.code = code;
    this.status = status;
  }
}

export function isApiErrorBody(value: unknown): value is ApiErrorBody {
  if (typeof value !== 'object' || value === null) return false;
  const error = (value as { error?: unknown }).error;
  return typeof error === 'object' && error !== null && typeof (error as { code?: unknown }).code === 'string';
}

/** Codes with their own copy in `errors.*`; anything else falls back to `errors.generic`. */
const KNOWN_CODES = new Set([
  'auth.invalid_credentials',
  'auth.invalid_code',
  'auth.email_taken',
  'auth.session_expired',
  'stepup.cancelled',
  'stepup.failed',
  'stepup.device_key_unavailable',
  'payout.insufficient_funds',
  'payout.creator_cannot_approve',
  'payout.invalid_state',
  'destination.not_found',
  'kyc.not_approved',
  'kyc.incomplete',
  'account.not_empty',
  'account.archived',
  'transfer.same_account',
  'cvu.already_exists',
  'cvu.alias_taken',
  'idempotency.conflict',
  'erp.invalid_credentials',
  'role.forbidden',
  'network.offline',
  'network.timeout',
]);

/** i18n key for any thrown value: a known API code, else the generic copy. */
export function errorMessageKey(error: unknown): string {
  if (error instanceof ApiError && KNOWN_CODES.has(error.code)) {
    return `errors.${error.code.replace('.', '_')}`;
  }
  return 'errors.generic';
}
