import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { Pressable, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { useToastStore } from '@/shared/stores/useToastStore';

import { AppText } from './AppText';
import { elevation, SURFACE_TONE } from './elevation';
import { Icon } from './Icon';

const TOAST_MS = 3_500;

/** Renders the toast raised by mutations (M2), above everything, dismissed after a few seconds. */
export function ToastHost() {
  const { t } = useTranslation();
  const insets = useSafeAreaInsets();
  const toast = useToastStore((state) => state.toast);
  const dismiss = useToastStore((state) => state.dismiss);

  useEffect(() => {
    if (!toast) return undefined;
    const timer = setTimeout(() => dismiss(toast.id), TOAST_MS);
    return () => clearTimeout(timer);
  }, [toast, dismiss]);

  if (!toast) return null;
  const icon = toast.kind === 'success' ? 'checkmark-circle' : toast.kind === 'error' ? 'alert-circle' : 'information-circle';
  const tone = toast.kind === 'success' ? 'success' : toast.kind === 'error' ? 'danger' : 'primary';
  return (
    <View pointerEvents="box-none" className="absolute inset-x-0 px-4" style={{ top: insets.top + 8 }}>
      <Pressable
        onPress={() => dismiss(toast.id)}
        accessibilityRole="alert"
        accessibilityLabel={toast.message}
        accessibilityHint={t('common.dismiss')}
        className={`flex-row items-center gap-3 rounded-card border border-border-subtle ${SURFACE_TONE.lg} px-4 py-3 ${elevation('lg')}`}
      >
        <Icon name={icon} size={22} tone={tone} />
        <AppText variant="label" className="flex-1">
          {toast.message}
        </AppText>
      </Pressable>
    </View>
  );
}
