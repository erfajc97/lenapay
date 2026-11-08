import type { PushPlatform } from '@/shared/api/types';

/**
 * Platform differences in one place (ME11). Keyboard avoidance: iOS pads the view, Android
 * resizes it (with edge-to-edge, `height` keeps the submit button above the keyboard).
 */
export function keyboardBehavior(os: string): 'padding' | 'height' | undefined {
  if (os === 'ios') return 'padding';
  if (os === 'android') return 'height';
  return undefined;
}

/** React Native's Modal fires `onDismiss` on iOS (react-native-web too); Android never does. */
export function modalReportsDismiss(os: string): boolean {
  return os !== 'android';
}

/** The contract's platform of a push token: the OS name when the contract knows it. */
export function pushPlatform(os: string): PushPlatform {
  return os === 'ios' || os === 'android' || os === 'web' ? os : 'unknown';
}

/** The floating tab bar blurs what scrolls behind it where blur is native (iOS, web); Android gets a solid surface. */
export function tabBarBlurs(os: string): boolean {
  return os === 'ios' || os === 'web';
}

/**
 * Where a sheet mounted at the root (step-up) presents. On iOS a root `Modal` presents from the
 * root view controller, which is already presenting any native modal screen (Mover, Pagar), so
 * UIKit refuses and the sheet never shows; there it goes to a window above everything instead.
 * Android's Modal is a dialog window that already sits above every screen.
 */
export function rootSheetUsesWindowOverlay(os: string): boolean {
  return os === 'ios';
}
