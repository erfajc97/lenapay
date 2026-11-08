import { Platform } from 'react-native';

import { api } from '@/shared/api';
import { biometricsAvailable } from '@/shared/biometrics/biometrics';
import { captureException } from '@/shared/reporting/reporting';
import { readDeviceKey, saveDeviceKey } from '@/shared/session/session';

import { generateDeviceKey } from './deviceKey';
import { ensureDeviceKey } from './stepUpFlow';

/** Registers this phone's step-up key after sign-in (see `ensureDeviceKey`); never throws. */
function registerThisDevice(): Promise<boolean> {
  return ensureDeviceKey({
    auth: api.auth,
    canUseBiometrics: biometricsAvailable,
    readDeviceKey,
    saveDeviceKey,
    generateKey: generateDeviceKey,
    deviceName: `lenapay-${Platform.OS}`,
  });
}

/** The device key is an upgrade of step-up: registering it never delays or fails the sign-in or the unlock. */
export function registerDeviceInBackground(): void {
  registerThisDevice().catch((error: unknown) => captureException(error, { area: 'device_registration' }));
}
