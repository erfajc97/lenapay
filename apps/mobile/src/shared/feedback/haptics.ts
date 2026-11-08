import * as Haptics from 'expo-haptics';

/** Haptic confirmation when money actually moved or was approved. Never blocks or throws. */
export function confirmHaptic(): void {
  Haptics.notificationAsync(Haptics.NotificationFeedbackType.Success).catch(() => undefined);
}

/** A light tick when the selection changes (tab bar). */
export function selectionHaptic(): void {
  Haptics.selectionAsync().catch(() => undefined);
}

export function warningHaptic(): void {
  Haptics.notificationAsync(Haptics.NotificationFeedbackType.Warning).catch(() => undefined);
}
