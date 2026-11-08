import { Platform } from 'react-native';

import { api } from '@/shared/api';
import { biometricsAvailable } from '@/shared/biometrics/biometrics';
import { readDeviceKey, saveDeviceKey } from '@/shared/session/session';

import { generateDeviceKey } from './deviceKey';
import { ensureDeviceKey } from './stepUpFlow';

/** Registers this phone's step-up key after sign-in (see `ensureDeviceKey`); never throws. */
export function registerThisDevice(): Promise<boolean> {
  return ensureDeviceKey({
    auth: api.auth,
    canUseBiometrics: biometricsAvailable,
    readDeviceKey,
    saveDeviceKey,
    generateKey: generateDeviceKey,
    deviceName: `lenapay-${Platform.OS}`,
  });
}
