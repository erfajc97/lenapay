import type { LenapayApi } from '@/shared/api/client';
import { ApiError } from '@/shared/api/errors';
import type { StepUpToken } from '@/shared/api/types';
import type { StoredDeviceKey } from '@/shared/session/session';

import { type DeviceKeyPair, signNonce } from './deviceKey';

export type StepUpInput = { method: 'device_key' } | { method: 'password'; password: string };

interface StepUpDeps {
  auth: Pick<LenapayApi['auth'], 'stepUpChallenge' | 'stepUpVerify'>;
  readDeviceKey(): Promise<StoredDeviceKey | null>;
}

/**
 * Challenge + answer (R-ID-3). Device key: the challenge names the device and the answer is
 * the Ed25519 signature of its nonce (the biometric prompt ran just before). Password: the
 * fallback. A device the API no longer knows reads as "no device key", so the sheet offers
 * the password instead of failing in a loop.
 */
export async function verifyStepUp(input: StepUpInput, deps: StepUpDeps): Promise<StepUpToken> {
  if (input.method === 'password') {
    const challenge = await deps.auth.stepUpChallenge({});
    return deps.auth.stepUpVerify({ challengeId: challenge.challengeId, method: 'password', password: input.password });
  }
  const key = await deps.readDeviceKey();
  if (!key) throw new ApiError('stepup.device_key_unavailable', 0);
  try {
    const challenge = await deps.auth.stepUpChallenge({ deviceId: key.deviceId });
    const signature = signNonce({ secretKey: key.secretKey, nonce: challenge.nonce });
    return await deps.auth.stepUpVerify({ challengeId: challenge.challengeId, method: 'device_signature', deviceId: key.deviceId, signature });
  } catch (error) {
    if (error instanceof ApiError && (error.code === 'device.not_found' || error.code === 'auth.challenge_invalid')) {
      throw new ApiError('stepup.device_key_unavailable', 0);
    }
    throw error;
  }
}

interface RegistrationDeps {
  auth: Pick<LenapayApi['auth'], 'registerDevice'>;
  canUseBiometrics(): Promise<boolean>;
  readDeviceKey(): Promise<StoredDeviceKey | null>;
  saveDeviceKey(key: StoredDeviceKey): Promise<void>;
  generateKey(): DeviceKeyPair;
  deviceName: string;
}

/**
 * After sign-in: a phone with biometrics and no device key registers one (POST /devices), so
 * the next money action confirms with Face ID / huella instead of the password. Returns
 * whether this phone now has a key. Never blocks the sign-in: a failure only means password.
 */
export async function ensureDeviceKey(deps: RegistrationDeps): Promise<boolean> {
  if (await deps.readDeviceKey()) return true;
  if (!(await deps.canUseBiometrics())) return false;
  const pair = deps.generateKey();
  try {
    const device = await deps.auth.registerDevice({ publicKey: pair.publicKey, name: deps.deviceName });
    await deps.saveDeviceKey({ deviceId: device.id, secretKey: pair.secretKey });
    return true;
  } catch {
    // Registration is an upgrade, not a requirement: step-up keeps the password path.
    return false;
  }
}
