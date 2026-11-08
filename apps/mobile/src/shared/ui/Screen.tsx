import { router } from 'expo-router';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { KeyboardAvoidingView, Platform, Pressable, RefreshControl, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { keyboardBehavior } from '@/shared/helpers/platform';
import { useIsOnline } from '@/shared/hooks/useIsOnline';
import { usePullToRefresh } from '@/shared/hooks/usePullToRefresh';

import { AppText } from './AppText';
import { Icon } from './Icon';
import { tabBarSpace } from './liquidTab';
import { Scroll } from './Scroll';

/**
 * The ONE screen wrapper (M10): safe-area insets with explicit edges, the header, the
 * offline banner (M6), keyboard avoidance and an optional sticky footer for the primary
 * action, so it is never covered by the keyboard.
 */
interface ScreenProps {
  children: ReactNode;
  title?: string;
  /** Show a back button (stack screens). */
  back?: boolean;
  /** Multi-step flows go one step back instead of leaving the screen. */
  onBack?: () => void;
  /** Tab screens: content scrolls under the floating tab bar and reserves its space at the end. */
  inTabs?: boolean;
  /** false for screens that render their own virtualized list. */
  scroll?: boolean;
  headerRight?: ReactNode;
  footer?: ReactNode;
  /** Pull-to-refresh: the spinner shows only while the person's pull runs (usePullToRefresh). */
  onRefresh?: () => Promise<unknown>;
}

function ScreenHeader({ title, back, onBack, right }: { title?: string; back?: boolean; onBack?: () => void; right?: ReactNode }) {
  const { t } = useTranslation();
  return (
    <View className="min-h-14 flex-row items-center gap-2 px-4">
      {back ? (
        <Pressable
          onPress={onBack ?? (() => (router.canGoBack() ? router.back() : router.replace('/')))}
          accessibilityRole="button"
          accessibilityLabel={t('common.back')}
          hitSlop={12}
          className="-ml-2 h-11 w-11 items-center justify-center rounded-button"
        >
          <Icon name="chevron-back" size={24} tone="text-primary" />
        </Pressable>
      ) : null}
      <AppText variant="heading" className="flex-1" numberOfLines={1} accessibilityRole="header">
        {title ?? ''}
      </AppText>
      {right}
    </View>
  );
}

function OfflineBanner() {
  const { t } = useTranslation();
  const online = useIsOnline();
  if (online) return null;
  return (
    <View className="mx-4 mb-2 flex-row items-center gap-2 rounded-chip bg-warning-bg px-3 py-2" accessibilityLiveRegion="polite">
      <Icon name="cloud-offline-outline" size={18} tone="warning" />
      <AppText variant="label" className="flex-1">
        {t('common.offline')}
      </AppText>
    </View>
  );
}

const noRefresh = () => Promise.resolve();

export function Screen({ children, title, back, onBack, inTabs, scroll = true, headerRight, footer, onRefresh }: ScreenProps) {
  const insets = useSafeAreaInsets();
  const pull = usePullToRefresh(onRefresh ?? noRefresh);
  const bottomInset = inTabs ? 0 : insets.bottom;
  return (
    <View className="flex-1 bg-bg-page" style={{ paddingTop: insets.top, paddingLeft: insets.left, paddingRight: insets.right }}>
      {title !== undefined || back ? <ScreenHeader title={title} back={back} onBack={onBack} right={headerRight} /> : null}
      <OfflineBanner />
      <KeyboardAvoidingView behavior={keyboardBehavior(Platform.OS)} className="flex-1">
        {scroll ? (
          <Scroll
            className="flex-1"
            contentContainerClassName="gap-4 px-4 pt-2"
            contentContainerStyle={{ paddingBottom: footer ? 24 : inTabs ? tabBarSpace(insets.bottom) : bottomInset + 32 }}
            refreshControl={onRefresh ? <RefreshControl refreshing={pull.refreshing} onRefresh={pull.onRefresh} tintColorClassName="accent-primary" /> : undefined}
          >
            {children}
          </Scroll>
        ) : (
          <View className="flex-1">{children}</View>
        )}
        {footer ? (
          <View className="border-t border-border-subtle bg-bg-surface px-4 pt-3" style={{ paddingBottom: Math.max(bottomInset, 16) }}>
            {footer}
          </View>
        ) : null}
      </KeyboardAvoidingView>
    </View>
  );
}
