import { useMutation } from '@tanstack/react-query';
import * as Clipboard from 'expo-clipboard';
import * as Sharing from 'expo-sharing';
import { Share } from 'react-native';

import type { LocalFile } from '@/shared/api/client';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';

/** Copy a CVU, alias or reference; the "Copiado" toast is this mutation's feedback (M2). */
export function useCopy() {
  return useMutation({
    mutationFn: (value: string) => Clipboard.setStringAsync(value),
    onSuccess: () => notifySuccess('common.copied'),
    onError: notifyError,
  });
}

/** System share sheet with a text (CVU + alias + holder). */
export function useShareText() {
  return useMutation({
    mutationFn: (message: string) => Share.share({ message }),
    onSuccess: () => undefined,
    onError: notifyError,
  });
}

/** System share sheet with a local file (proof of payment PDF). */
export function useShareFile() {
  return useMutation({
    mutationFn: async (load: () => Promise<LocalFile>) => {
      const file = await load();
      await Sharing.shareAsync(file.uri, { mimeType: file.mimeType, UTI: 'com.adobe.pdf' });
    },
    onSuccess: () => undefined,
    onError: notifyError,
  });
}
