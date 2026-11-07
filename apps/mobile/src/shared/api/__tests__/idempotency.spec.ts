import { describe, expect, it } from 'vitest';

import { IDEMPOTENCY_TTL_MS, idempotencyKeyFor, releaseIdempotencyKey } from '../idempotency';

// Why: a payout retried after a timeout must carry the SAME key or the money leaves twice
// (MO2); a different payload must not reuse it or the API answers 409 to a new payout.
describe('idempotencyKeyFor', () => {
  const payload = { accountId: 'acc_main', amount: 150_000_00, currency: 'ARS', to: 'hormigon.plata' };
  const now = 1_700_000_000_000;

  it('reuses the key for the same scope and payload, whatever the key order', async () => {
    const first = await idempotencyKeyFor('payout:a', payload, now);
    const reordered = { to: 'hormigon.plata', currency: 'ARS', amount: 150_000_00, accountId: 'acc_main' };
    expect(await idempotencyKeyFor('payout:a', reordered, now + 1_000)).toBe(first);
  });

  it('issues a new key when the payload changes', async () => {
    const first = await idempotencyKeyFor('payout:b', payload, now);
    expect(await idempotencyKeyFor('payout:b', { ...payload, amount: 150_000_01 }, now)).not.toBe(first);
  });

  it('keeps scopes apart', async () => {
    expect(await idempotencyKeyFor('payout:c', payload, now)).not.toBe(await idempotencyKeyFor('transfer:c', payload, now));
  });

  it('expires at 24 h: last ms reuses, first ms after issues a new key', async () => {
    const first = await idempotencyKeyFor('payout:d', payload, now);
    expect(await idempotencyKeyFor('payout:d', payload, now + IDEMPOTENCY_TTL_MS - 1)).toBe(first);
    expect(await idempotencyKeyFor('payout:d', payload, now + IDEMPOTENCY_TTL_MS)).not.toBe(first);
  });

  it('issues a new key once the previous operation was answered', async () => {
    const first = await idempotencyKeyFor('payout:e', payload, now);
    await releaseIdempotencyKey('payout:e');
    expect(await idempotencyKeyFor('payout:e', payload, now)).not.toBe(first);
  });
});
