import * as ed from '@noble/ed25519';
import { describe, expect, it, vi } from 'vitest';

import { ApiError } from '@/shared/api/errors';
import type { RegisterDeviceRequest, StepUpChallengeRequest, StepUpVerifyRequest } from '@/shared/api/types';

import { fromBase64Url, generateDeviceKey, signNonce, toBase64Url } from '../deviceKey';
import { ensureDeviceKey, verifyStepUp } from '../stepUpFlow';

// Why: the API verifies the step-up signature as Ed25519 over the raw nonce bytes with the
// registered public key (base64url, no padding); one wrong byte or a padded encoding and every
// biometric confirmation fails, sending money actions back to the password.
describe('device key', () => {
  it('round-trips base64url without padding for every length', () => {
    for (const length of [0, 1, 2, 3, 31, 32, 64]) {
      const bytes = Uint8Array.from({ length }, (_, index) => (index * 37 + 11) % 256);
      const encoded = toBase64Url(bytes);
      expect(encoded).toMatch(/^[A-Za-z0-9_-]*$/);
      expect(Array.from(fromBase64Url(encoded))).toEqual(Array.from(bytes));
    }
    expect(toBase64Url(Uint8Array.from([251, 255]))).toBe('-_8');
  });

  it('rejects text that is not base64url', () => {
    expect(() => fromBase64Url('ab+c')).toThrow('invalid base64url');
    expect(() => fromBase64Url('abcde')).toThrow('invalid base64url');
  });

  it('signs the nonce so the registered public key verifies it, and nothing else', () => {
    const pair = generateDeviceKey();
    expect(fromBase64Url(pair.publicKey)).toHaveLength(32);
    const nonce = toBase64Url(Uint8Array.from({ length: 32 }, (_, index) => index));
    const signature = fromBase64Url(signNonce({ secretKey: pair.secretKey, nonce }));
    expect(signature).toHaveLength(64);
    expect(ed.verify(signature, fromBase64Url(nonce), fromBase64Url(pair.publicKey))).toBe(true);
    expect(ed.verify(signature, fromBase64Url(toBase64Url(Uint8Array.from({ length: 32 }, () => 7))), fromBase64Url(pair.publicKey))).toBe(false);
  });

  it('generates a different key each time', () => {
    expect(generateDeviceKey().secretKey).not.toBe(generateDeviceKey().secretKey);
  });
});

function authFake() {
  return {
    stepUpChallenge: vi.fn(async (_body: StepUpChallengeRequest) => ({ challengeId: 'ch-1', nonce: toBase64Url(new Uint8Array(32)), expiresAt: '2026-11-07T12:02:00Z' })),
    stepUpVerify: vi.fn(async (_body: StepUpVerifyRequest) => ({ stepUpToken: 'su-1', expiresAt: '2026-11-07T12:05:00Z' })),
  };
}

describe('verifyStepUp', () => {
  it('answers a password challenge without naming a device', async () => {
    const auth = authFake();
    await expect(verifyStepUp({ method: 'password', password: 'Lenapay2026!' }, { auth, readDeviceKey: async () => null })).resolves.toMatchObject({ stepUpToken: 'su-1' });
    expect(auth.stepUpChallenge).toHaveBeenCalledWith({});
    expect(auth.stepUpVerify).toHaveBeenCalledWith({ challengeId: 'ch-1', method: 'password', password: 'Lenapay2026!' });
  });

  it('signs a device challenge with the stored key', async () => {
    const auth = authFake();
    const pair = generateDeviceKey();
    await verifyStepUp({ method: 'device_key' }, { auth, readDeviceKey: async () => ({ deviceId: 'dev-1', secretKey: pair.secretKey }) });
    expect(auth.stepUpChallenge).toHaveBeenCalledWith({ deviceId: 'dev-1' });
    const body = auth.stepUpVerify.mock.calls[0]?.[0];
    expect(body).toMatchObject({ challengeId: 'ch-1', method: 'device_signature', deviceId: 'dev-1' });
    expect(ed.verify(fromBase64Url(body?.signature ?? ''), new Uint8Array(32), fromBase64Url(pair.publicKey))).toBe(true);
  });

  it('falls back to the password when the phone has no key or the API forgot it', async () => {
    await expect(verifyStepUp({ method: 'device_key' }, { auth: authFake(), readDeviceKey: async () => null })).rejects.toMatchObject({ code: 'stepup.device_key_unavailable' });
    const forgotten = authFake();
    forgotten.stepUpChallenge.mockRejectedValueOnce(new ApiError('device.not_found', 404));
    const key = { deviceId: 'dev-1', secretKey: generateDeviceKey().secretKey };
    await expect(verifyStepUp({ method: 'device_key' }, { auth: forgotten, readDeviceKey: async () => key })).rejects.toMatchObject({ code: 'stepup.device_key_unavailable' });
  });

  it('keeps other failures as they are (offline is not "no key")', async () => {
    const auth = authFake();
    auth.stepUpVerify.mockRejectedValueOnce(new ApiError('network.offline', 0));
    const key = { deviceId: 'dev-1', secretKey: generateDeviceKey().secretKey };
    await expect(verifyStepUp({ method: 'device_key' }, { auth, readDeviceKey: async () => key })).rejects.toMatchObject({ code: 'network.offline' });
  });
});

describe('ensureDeviceKey', () => {
  function deps(over: Partial<Parameters<typeof ensureDeviceKey>[0]> = {}) {
    const registerDevice = vi.fn(async (_body: RegisterDeviceRequest) => ({ id: 'dev-9', name: 'lenapay-ios', createdAt: '2026-11-07T12:00:00Z' }));
    return {
      registerDevice,
      auth: { registerDevice },
      canUseBiometrics: async () => true,
      readDeviceKey: async () => null,
      saveDeviceKey: vi.fn(async () => undefined),
      generateKey: () => ({ secretKey: 'c2VjcmV0', publicKey: 'cHVibGlj' }),
      deviceName: 'lenapay-ios',
      ...over,
    };
  }

  it('registers the public key and stores the id with the secret', async () => {
    const d = deps();
    await expect(ensureDeviceKey(d)).resolves.toBe(true);
    expect(d.registerDevice).toHaveBeenCalledWith({ publicKey: 'cHVibGlj', name: 'lenapay-ios' });
    expect(d.saveDeviceKey).toHaveBeenCalledWith({ deviceId: 'dev-9', secretKey: 'c2VjcmV0' });
  });

  it('does nothing when the phone already has a key or no biometrics', async () => {
    const existing = deps({ readDeviceKey: async () => ({ deviceId: 'dev-1', secretKey: 'x' }) });
    await expect(ensureDeviceKey(existing)).resolves.toBe(true);
    expect(existing.registerDevice).not.toHaveBeenCalled();
    const noBiometrics = deps({ canUseBiometrics: async () => false });
    await expect(ensureDeviceKey(noBiometrics)).resolves.toBe(false);
    expect(noBiometrics.registerDevice).not.toHaveBeenCalled();
  });

  it('never fails the sign-in when the registration fails, and stores nothing', async () => {
    const d = deps();
    d.registerDevice.mockRejectedValueOnce(new ApiError('network.offline', 0));
    await expect(ensureDeviceKey(d)).resolves.toBe(false);
    expect(d.saveDeviceKey).not.toHaveBeenCalled();
  });
});
