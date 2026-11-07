import * as LocalAuthentication from 'expo-local-authentication';
import { Platform } from 'react-native';

/**
 * Device biometrics (Face ID / huella). Unavailable, not enrolled or web → false, so callers
 * fall back to the password, never to a dead end.
 */
export async function biometricsAvailable(): Promise<boolean> {
  if (Platform.OS === 'web') return false;
  try {
    return (await LocalAuthentication.hasHardwareAsync()) && (await LocalAuthentication.isEnrolledAsync());
  } catch {
    return false;
  }
}

export async function authenticateWithBiometrics(params: { prompt: string; cancelLabel: string }): Promise<boolean> {
  try {
    const result = await LocalAuthentication.authenticateAsync({ promptMessage: params.prompt, cancelLabel: params.cancelLabel });
    return result.success;
  } catch {
    return false;
  }
}
