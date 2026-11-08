import { type ReactNode, useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { Modal, Platform, Pressable, View } from 'react-native';
import Animated, { FadeInUp } from 'react-native-reanimated';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { modalReportsDismiss } from '@/shared/helpers/platform';

import { elevation, SURFACE_TONE } from './elevation';

/** Distance from the top safe-area edge to the menu: just under a header avatar. */
const ANCHOR_TOP = 60;

/**
 * A menu anchored to the top-right header control (elevation lg), dismissed by an outside tap
 * or Android back. Like Sheet, `onDismissed` runs once it is off screen: navigate there, never
 * while it is still fading out (iOS leaves an invisible presentation that swallows touches).
 */
export function Popover({
  visible,
  label,
  onClose,
  onDismissed,
  children,
}: {
  visible: boolean;
  label: string;
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
    <Modal visible={visible} transparent animationType="fade" onRequestClose={onClose} onDismiss={onDismissed} statusBarTranslucent navigationBarTranslucent>
      <Pressable className="absolute inset-0 bg-overlay" onPress={onClose} accessibilityRole="button" accessibilityLabel={t('common.close')} />
      <View pointerEvents="box-none" className="absolute inset-0" style={{ paddingTop: insets.top + ANCHOR_TOP, paddingLeft: insets.left + 16, paddingRight: insets.right + 16 }}>
        <Animated.View
          entering={FadeInUp.duration(160)}
          accessibilityRole="menu"
          accessibilityLabel={label}
          className={`ml-auto w-full max-w-[320px] overflow-hidden rounded-card py-1 ${SURFACE_TONE.lg} ${elevation('lg')}`}
        >
          {children}
        </Animated.View>
      </View>
    </Modal>
  );
}
