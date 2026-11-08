import * as Notifications from 'expo-notifications';
import { Linking, Platform } from 'react-native';

import { api, isMockApi } from '@/shared/api';
import { config } from '@/shared/config/env';
import { pushPlatform } from '@/shared/helpers/platform';

/**
 * Push (R-NOT-1). Asked only from Settings after an explanation (ME13). In `mock` mode the
 * permission is still requested (to preview the flow) but no Expo token is fetched: that
 * needs an EAS project id the app does not have yet.
 */
type PushResult = 'enabled' | 'denied' | 'unavailable';

export async function enablePush(): Promise<PushResult> {
  if (Platform.OS === 'web') return 'unavailable';
  const current = await Notifications.getPermissionsAsync();
  const status = current.granted ? current : await Notifications.requestPermissionsAsync();
  if (!status.granted) return 'denied';
  if (isMockApi() || !config.easProjectId) return 'enabled';
  const token = await Notifications.getExpoPushTokenAsync({ projectId: config.easProjectId });
  await api.notifications.registerPushToken(token.data, pushPlatform(Platform.OS));
  return 'enabled';
}

/** Denied permission leads somewhere: the system settings of the app (ME13). */
export function openNotificationSettings(): void {
  void Linking.openSettings();
}
