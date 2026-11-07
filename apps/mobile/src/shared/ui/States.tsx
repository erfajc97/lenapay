import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Animated, View } from 'react-native';

import { errorMessageKey } from '@/shared/api/errors';

import { AppText } from './AppText';
import { Button } from './Button';
import { Card } from './Card';
import { Icon, type IconName } from './Icon';

/** Pulsing placeholder (M6 loading state). JS driver: a looping native-driven animation can stall Fabric commits on iOS. */
export function Skeleton({ className }: { className?: string }) {
  const [opacity] = useState(() => new Animated.Value(0.5));
  useEffect(() => {
    const loop = Animated.loop(
      Animated.sequence([
        Animated.timing(opacity, { toValue: 1, duration: 700, useNativeDriver: false }),
        Animated.timing(opacity, { toValue: 0.5, duration: 700, useNativeDriver: false }),
      ]),
    );
    loop.start();
    return () => loop.stop();
  }, [opacity]);
  return (
    <Animated.View style={{ opacity }}>
      <View className={`rounded-chip bg-border-subtle ${className ?? ''}`} />
    </Animated.View>
  );
}

/** A list-shaped skeleton: fixed placeholder rows, keyed by index on purpose (ME8). */
export function SkeletonList({ rows = 5 }: { rows?: number }) {
  const { t } = useTranslation();
  return (
    <View className="gap-3" accessible accessibilityLabel={t('common.loading')}>
      {Array.from({ length: rows }, (_, index) => (
        <Card key={index} className="flex-row items-center gap-3">
          <Skeleton className="h-10 w-10 rounded-full" />
          <View className="flex-1 gap-2">
            <Skeleton className="h-4 w-2/3" />
            <Skeleton className="h-3 w-1/3" />
          </View>
          <Skeleton className="h-4 w-20" />
        </Card>
      ))}
    </View>
  );
}

export function EmptyState({ icon, title, body, actionLabel, onAction }: { icon: IconName; title: string; body?: string; actionLabel?: string; onAction?: () => void }) {
  return (
    <View className="items-center gap-3 px-6 py-10">
      <View className="h-14 w-14 items-center justify-center rounded-full bg-primary-bg">
        <Icon name={icon} size={26} tone="primary" />
      </View>
      <AppText variant="subheading" className="text-center">
        {title}
      </AppText>
      {body ? (
        <AppText tone="secondary" className="text-center">
          {body}
        </AppText>
      ) : null}
      {actionLabel && onAction ? <Button label={actionLabel} onPress={onAction} compact /> : null}
    </View>
  );
}

/** Error with retry (M6): the copy comes from the error code, never the server message. */
export function ErrorState({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const { t } = useTranslation();
  return (
    <View className="items-center gap-3 px-6 py-10" accessibilityLiveRegion="polite">
      <View className="h-14 w-14 items-center justify-center rounded-full bg-danger-bg">
        <Icon name="alert-circle-outline" size={26} tone="danger" />
      </View>
      <AppText variant="subheading" className="text-center">
        {t('common.errorTitle')}
      </AppText>
      <AppText tone="secondary" className="text-center">
        {t(errorMessageKey(error))}
      </AppText>
      <Button label={t('common.retry')} onPress={onRetry} variant="secondary" icon="refresh" compact />
    </View>
  );
}
