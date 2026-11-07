import * as Haptics from 'expo-haptics';

/** Haptic confirmation when money actually moved or was approved. Never blocks or throws. */
export function confirmHaptic(): void {
  Haptics.notificationAsync(Haptics.NotificationFeedbackType.Success).catch(() => undefined);
}

export function warningHaptic(): void {
  Haptics.notificationAsync(Haptics.NotificationFeedbackType.Warning).catch(() => undefined);
}
