import { describe, expect, it } from 'vitest';

import { ApiError } from '@/shared/api/errors';

import { connectionChip, describeConnectionCheck, firewallRuleCommand } from '../connectionCheck';

// Why: the developer screen is opened exactly when the phone cannot reach the API. Each
// failure must say what to do next (same Wi-Fi, Windows firewall, Local Network permission,
// wrong port), and a stale "Conectado" for a URL that was never tested is a lie.
const URL = 'http://192.168.68.53:18080';

describe('firewallRuleCommand', () => {
  it('opens the exact port of the tested URL', () => {
    expect(firewallRuleCommand(URL)).toBe('New-NetFirewallRule -DisplayName "Lenapay API 18080" -Direction Inbound -Protocol TCP -LocalPort 18080 -Action Allow');
    expect(firewallRuleCommand('http://192.168.1.4:9000/api')).toContain('-LocalPort 9000 ');
  });

  it('falls back to the scheme port when the URL has none', () => {
    expect(firewallRuleCommand('http://pc.local')).toContain('-LocalPort 80 ');
    expect(firewallRuleCommand('https://pc.local')).toContain('-LocalPort 443 ');
  });
});

describe('describeConnectionCheck', () => {
  it('shows the green card with latency and the /healthz status on success', () => {
    expect(describeConnectionCheck({ url: URL, outcome: { kind: 'success', status: 'ok', latencyMs: 42 } })).toEqual({
      tone: 'success',
      titleKey: 'developer.connection.successTitle',
      bodyKey: 'developer.connection.successBody',
      params: { ms: '42', status: 'ok', port: '18080', seconds: '5' },
      firewallCommand: null,
    });
  });

  it('asks about the Wi-Fi and the firewall on a timeout, with the copyable command', () => {
    const view = describeConnectionCheck({ url: URL, outcome: { kind: 'failure', error: new ApiError('network.timeout', 0) } });
    expect(view.tone).toBe('danger');
    expect(view.titleKey).toBe('developer.connection.timeoutTitle');
    expect(view.bodyKey).toBe('developer.connection.timeoutBody');
    expect(view.firewallCommand).toBe(firewallRuleCommand(URL));
    expect(view.params).toMatchObject({ port: '18080', seconds: '5' });
  });

  it('points at the address, the API and the Local Network permission when nothing answers', () => {
    const view = describeConnectionCheck({ url: URL, outcome: { kind: 'failure', error: new ApiError('network.offline', 0) } });
    expect(view.titleKey).toBe('developer.connection.unreachableTitle');
    expect(view.bodyKey).toBe('developer.connection.unreachableBody');
    expect(view.firewallCommand).toBe(firewallRuleCommand(URL));
  });

  it('explains an invalid URL without a firewall command', () => {
    const view = describeConnectionCheck({ url: 'localhost', outcome: { kind: 'failure', error: new ApiError('health.invalid_url', 0) } });
    expect(view.titleKey).toBe('developer.connection.invalidUrlTitle');
    expect(view.firewallCommand).toBeNull();
  });

  it('names the HTTP status of a non-200 answer', () => {
    const view = describeConnectionCheck({ url: URL, outcome: { kind: 'failure', error: new ApiError('http.404', 404) } });
    expect(view.titleKey).toBe('developer.connection.httpTitle');
    expect(view.bodyKey).toBe('developer.connection.httpBody');
    expect(view.params.status).toBe('404');
    expect(view.firewallCommand).toBeNull();
  });

  it('says the database is down on a 503 (the API itself answered)', () => {
    const view = describeConnectionCheck({ url: URL, outcome: { kind: 'failure', error: new ApiError('provider.unavailable', 503) } });
    expect(view.titleKey).toBe('developer.connection.httpTitle');
    expect(view.bodyKey).toBe('developer.connection.httpUnavailableBody');
    expect(view.params.status).toBe('503');
  });

  it('treats a 200 that is not {status:"ok"} as another server', () => {
    const view = describeConnectionCheck({ url: URL, outcome: { kind: 'failure', error: new ApiError('health.unexpected_body', 200) } });
    expect(view.titleKey).toBe('developer.connection.notLenapayTitle');
    expect(view.firewallCommand).toBeNull();
  });

  it('falls back to a generic retry for anything else', () => {
    const view = describeConnectionCheck({ url: URL, outcome: { kind: 'failure', error: new Error('boom') } });
    expect(view.titleKey).toBe('developer.connection.unexpectedTitle');
    expect(view.bodyKey).toBe('developer.connection.unexpectedBody');
  });
});

describe('connectionChip', () => {
  it('is "untested" before any check', () => {
    expect(connectionChip({ pending: false, testedUrl: undefined, currentUrl: URL, succeeded: false, failed: false })).toBe('untested');
  });

  it('is "checking" while the check runs', () => {
    expect(connectionChip({ pending: true, testedUrl: URL, currentUrl: URL, succeeded: false, failed: false })).toBe('checking');
  });

  it('is "connected" or "offline" for the URL that was tested', () => {
    expect(connectionChip({ pending: false, testedUrl: URL, currentUrl: URL, succeeded: true, failed: false })).toBe('connected');
    expect(connectionChip({ pending: false, testedUrl: URL, currentUrl: URL, succeeded: false, failed: true })).toBe('offline');
  });

  it('goes back to "untested" once the URL is edited (same URL modulo trailing slash still counts)', () => {
    expect(connectionChip({ pending: false, testedUrl: URL, currentUrl: 'http://192.168.68.54:18080', succeeded: true, failed: false })).toBe('untested');
    expect(connectionChip({ pending: false, testedUrl: URL, currentUrl: `${URL}/ `, succeeded: true, failed: false })).toBe('connected');
  });
});
