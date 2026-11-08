import { ApiError } from '@/shared/api/errors';
import { HEALTH_CHECK_TIMEOUT_MS, type HealthCheckResult } from '@/shared/api/live/health';
import { resolveApiUrl } from '@/shared/config/env';

/**
 * What the developer screen says after "Probar conexión": every outcome maps to copy that
 * names the cause and the next step. The screen is opened precisely when the phone cannot
 * reach the PC, so a failure without a next step is a dead end.
 */
export type ConnectionOutcome = ({ kind: 'success' } & HealthCheckResult) | { kind: 'failure'; error: unknown };

export interface ConnectionCheckView {
  tone: 'success' | 'danger';
  titleKey: string;
  bodyKey: string;
  params: Record<string, string>;
  /** The PowerShell rule that opens the port on the PC, when the firewall may be the cause. */
  firewallCommand: string | null;
}

export type ConnectionChipState = 'untested' | 'checking' | 'connected' | 'offline';

const KEY = 'developer.connection';

function portOf(url: string): string {
  const match = /^(https?):\/\/[^/:?#]+(?::(\d+))?/.exec(url.trim());
  if (match?.[2]) return match[2];
  return match?.[1] === 'https' ? '443' : '80';
}

/** Windows Firewall drops LAN traffic to the API port by default; this rule lets it in. */
export function firewallRuleCommand(url: string): string {
  const port = portOf(url);
  return `New-NetFirewallRule -DisplayName "Lenapay API ${port}" -Direction Inbound -Protocol TCP -LocalPort ${port} -Action Allow`;
}

function failureView(url: string, error: unknown, params: Record<string, string>): ConnectionCheckView {
  const view = (name: string, body = name, firewall = false): ConnectionCheckView => ({
    tone: 'danger',
    titleKey: `${KEY}.${name}Title`,
    bodyKey: `${KEY}.${body}Body`,
    params,
    firewallCommand: firewall ? firewallRuleCommand(url) : null,
  });
  if (!(error instanceof ApiError)) return view('unexpected');
  if (error.code === 'health.invalid_url') return view('invalidUrl');
  if (error.code === 'network.timeout') return view('timeout', 'timeout', true);
  if (error.code === 'network.offline') return view('unreachable', 'unreachable', true);
  if (error.code === 'health.unexpected_body') return view('notLenapay');
  if (error.status > 0) {
    // 503 is the API answering that its database does not (provider.unavailable).
    return { ...view('http', error.status === 503 ? 'httpUnavailable' : 'http'), params: { ...params, status: String(error.status) } };
  }
  return view('unexpected');
}

export function describeConnectionCheck(params: { url: string; outcome: ConnectionOutcome }): ConnectionCheckView {
  const base = { port: portOf(params.url), seconds: String(HEALTH_CHECK_TIMEOUT_MS / 1_000) };
  const { outcome } = params;
  if (outcome.kind === 'failure') return failureView(params.url, outcome.error, base);
  return {
    tone: 'success',
    titleKey: `${KEY}.successTitle`,
    bodyKey: `${KEY}.successBody`,
    params: { ...base, ms: String(outcome.latencyMs), status: outcome.status },
    firewallCommand: null,
  };
}

/** The chip at the top: a result only counts for the URL that was actually tested. */
export function connectionChip(params: { pending: boolean; testedUrl: string | undefined; currentUrl: string; succeeded: boolean; failed: boolean }): ConnectionChipState {
  if (params.pending) return 'checking';
  if (params.testedUrl === undefined || params.testedUrl !== resolveApiUrl(params.currentUrl, undefined)) return 'untested';
  if (params.succeeded) return 'connected';
  return params.failed ? 'offline' : 'untested';
}
