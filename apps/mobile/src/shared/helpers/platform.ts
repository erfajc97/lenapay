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
