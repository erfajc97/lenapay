import NetInfo from '@react-native-community/netinfo';
import { focusManager, onlineManager } from '@tanstack/react-query';
import { AppState, Platform } from 'react-native';

/**
 * React Native has no window focus/online events: wire TanStack Query to AppState and
 * NetInfo ONCE at the root (M3), so data refetches when the app comes back to the
 * foreground or the connection returns.
 */
export function wireQueryLifecycle(): void {
  onlineManager.setEventListener((setOnline) =>
    NetInfo.addEventListener((state) => {
      setOnline(state.isConnected !== false);
    }),
  );
  // On web the library already listens to the window's visibility events.
  if (Platform.OS === 'web') return;
  focusManager.setEventListener((handleFocus) => {
    const subscription = AppState.addEventListener('change', (status) => handleFocus(status === 'active'));
    return () => subscription.remove();
  });
}
