import { useMutation, useQueryClient } from '@tanstack/react-query';

import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { enablePush, openNotificationSettings } from '@/shared/notifications/push';
import { setBiometricsEnabled } from '@/shared/session/session';

import { BIOMETRICS_SETTING_KEY } from '../types';

export function useToggleBiometrics() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: setBiometricsEnabled,
    onSuccess: (_, enabled) => {
      queryClient.setQueryData(BIOMETRICS_SETTING_KEY, enabled);
      notifySuccess(enabled ? 'security.biometricsOn' : 'security.biometricsOff');
    },
    onError: notifyError,
  });
}

/** Push permission is asked here, after the in-app explanation (ME13). */
export function useEnablePush() {
  return useMutation({
    mutationFn: enablePush,
    onSuccess: (result) => {
      if (result === 'denied') openNotificationSettings();
      notifySuccess(`security.push.${result}`);
    },
    onError: notifyError,
  });
}
