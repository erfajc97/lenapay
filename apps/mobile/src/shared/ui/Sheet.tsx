import { BlurView as ExpoBlurView } from 'expo-blur';
import { type ReactNode, useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { KeyboardAvoidingView, Modal, Platform, Pressable, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { withUniwind } from 'uniwind';

import { keyboardBehavior, modalReportsDismiss } from '@/shared/helpers/platform';

import { AppText } from './AppText';
import { elevation, SURFACE_TONE } from './elevation';
import { Icon } from './Icon';
import { Scroll } from './Scroll';

const BlurView = withUniwind(ExpoBlurView);

/**
 * Bottom sheet (radius 24, elevation xl) over a blurred, dimmed backdrop. The blur is cheap
 * where it is native (iOS, web); Android keeps expo-blur's default translucent fallback.
 * `onClose` runs for the backdrop, the close button and Android hardware back (ME11).
 * `onDismissed` runs once the sheet is fully off screen: navigate there, never while it is
 * still sliding out (on iOS that leaves an invisible presentation that swallows every touch).
 */
export function Sheet({
  visible,
  title,
  onClose,
  onDismissed,
  children,
}: {
  visible: boolean;
  title: string;
  onClose: () => void;
  onDismissed?: () => void;
  children: ReactNode;
}) {
  const { t } = useTranslation();
  const insets = useSafeAreaInsets();
  const wasVisible = useRef(visible);

  // Android's Modal has no onDismiss: its dialog is gone once `visible` turns false.
  useEffect(() => {
    if (wasVisible.current && !visible && !modalReportsDismiss(Platform.OS)) onDismissed?.();
    wasVisible.current = visible;
  }, [visible, onDismissed]);

  return (
    <Modal
      visible={visible}
      transparent
      animationType="slide"
      onRequestClose={onClose}
      onDismiss={onDismissed}
      statusBarTranslucent
      navigationBarTranslucent
    >
      <KeyboardAvoidingView behavior={keyboardBehavior(Platform.OS)} className="flex-1 justify-end">
        <BlurView intensity={18} tint="default" className="absolute inset-0" />
        <Pressable className="absolute inset-0 bg-overlay" onPress={onClose} accessibilityRole="button" accessibilityLabel={t('common.close')} />
        <View className={`max-h-[90%] rounded-t-sheet ${SURFACE_TONE.xl} ${elevation('xl')}`} style={{ paddingBottom: Math.max(insets.bottom, 16) }}>
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
