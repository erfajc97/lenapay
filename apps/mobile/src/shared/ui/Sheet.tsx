import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { KeyboardAvoidingView, Modal, Platform, Pressable, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { keyboardBehavior } from '@/shared/helpers/platform';

import { AppText } from './AppText';
import { Icon } from './Icon';
import { Scroll } from './Scroll';

/**
 * Bottom sheet (radius 16) over a dimmed backdrop. `onClose` runs for the backdrop, the close
 * button and Android hardware back (ME11).
 */
export function Sheet({ visible, title, onClose, children }: { visible: boolean; title: string; onClose: () => void; children: ReactNode }) {
  const { t } = useTranslation();
  const insets = useSafeAreaInsets();
  return (
    <Modal visible={visible} transparent animationType="slide" onRequestClose={onClose} statusBarTranslucent navigationBarTranslucent>
      <KeyboardAvoidingView behavior={keyboardBehavior(Platform.OS)} className="flex-1 justify-end">
        <Pressable className="absolute inset-0 bg-overlay" onPress={onClose} accessibilityRole="button" accessibilityLabel={t('common.close')} />
        <View className="max-h-[90%] rounded-t-sheet bg-bg-surface" style={{ paddingBottom: Math.max(insets.bottom, 16) }}>
          <View className="items-center pt-2">
            <View className="h-1 w-10 rounded-full bg-border-default" />
          </View>
          <View className="flex-row items-center gap-2 px-4 pb-2 pt-3">
            <AppText variant="subheading" className="flex-1" accessibilityRole="header">
              {title}
            </AppText>
            <Pressable onPress={onClose} accessibilityRole="button" accessibilityLabel={t('common.close')} hitSlop={12} className="h-11 w-11 items-center justify-center">
              <Icon name="close" size={22} tone="text-secondary" />
            </Pressable>
          </View>
          <Scroll contentContainerClassName="gap-4 px-4 pb-2">{children}</Scroll>
        </View>
      </KeyboardAvoidingView>
    </Modal>
  );
}
