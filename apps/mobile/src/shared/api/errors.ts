import type { ApiErrorBody } from './types';

/**
 * Every failure the app shows comes through this class: the `code` is the contract's error
 * code (`ledger.insufficient_funds`), mapped to i18n copy by `errorMessageKey`. The server
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

/**
 * Codes with their own copy in `errors.*` (`area.reason` → `errors.area_reason`); anything
 * else falls back to `errors.generic`. The API's catalog (apps/api/internal/platform/apperr)
 * minus codes the app never meets (webhooks, internal routes), plus the client's own codes.
 */
export const ERROR_CODES = [
  // Client-side
  'auth.session_expired',
  'stepup.cancelled',
  'stepup.failed',
  'stepup.device_key_unavailable',
  'network.offline',
  'network.timeout',
  // Platform
  'validation.failed',
  'request.too_large',
  'request.timeout',
  'request.rate_limited',
  'resource.not_found',
  'resource.conflict',
  'operation.in_progress',
  // Identity
  'auth.forbidden',
  'auth.step_up_required',
  'auth.invalid_credentials',
  'auth.mfa_required',
  'auth.email_not_verified',
  'auth.email_taken',
  'auth.code_invalid',
  'auth.challenge_invalid',
  'device.not_found',
  'idempotency.key_reused',
  // Orgs and team
  'org.not_found',
  'member.already_exists',
  'member.not_found',
  'user.not_found',
  'invite.not_found',
  'invite.expired',
  'role.escalation_forbidden',
  'org.last_admin',
  'org.personal_mode_disabled',
  // KYC
  'kyc.not_approved',
  'kyc.case_not_found',
  'kyc.invalid_state',
  'kyc.incomplete',
  'kyc.document_invalid',
  // Money, accounts, ledger
  'money.invalid_amount',
  'money.currency_mismatch',
  'account.not_found',
  'account.archived',
  'account.not_empty',
  'account.not_archivable',
  'ledger.insufficient_funds',
  'ledger.hold_not_active',
  'transfer.same_account',
  'movement.not_found',
  // Payers, CVUs, payins, destinations
  'payer.not_found',
  'cvu.not_found',
  'cvu.already_exists',
  'cvu.alias_taken',
  'cvu.disabled',
  'payin.not_found',
  'provider.unavailable',
  'destination.not_found',
  'destination.invalid',
  // Payouts
  'payout.not_found',
  'payout.invalid_state',
  'payout.self_approval_forbidden',
  'payout.proof_unavailable',
  'payout.attachment_invalid',
  // Reconciliation and ERP
  'reconciliation.run_not_found',
  'erp_link.not_found',
  'erp_link.no_candidate',
  'erp_link.credentials_invalid',
  'erp_link.already_linked',
  // Mis pagos
  'payer_link.not_found',
  'installment.not_found',
] as const;

const KNOWN_CODES: ReadonlySet<string> = new Set(ERROR_CODES);

/** i18n key of an error code: `ledger.insufficient_funds` → `errors.ledger_insufficient_funds`. */
function errorCodeKey(code: string): string {
  return `errors.${code.replace('.', '_')}`;
}

/** i18n key for any thrown value: a known API code, else the generic copy. */
export function errorMessageKey(error: unknown): string {
  if (error instanceof ApiError && KNOWN_CODES.has(error.code)) return errorCodeKey(error.code);
  return 'errors.generic';
}
