import { Image as ExpoImage } from 'expo-image';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { StyleSheet, View } from 'react-native';
import Animated, { Easing, useAnimatedStyle, useReducedMotion, useSharedValue, withDelay, withTiming } from 'react-native-reanimated';
import { withUniwind } from 'uniwind';

import { AppText } from './AppText';
import { LEBANE_ISOTYPE, LEBANE_WORDMARK_ON_DARK, LEBANE_WORDMARK_ON_LIGHT } from './brandAssets';
import { launchHoldMs } from './launchTiming';

const Image = withUniwind(ExpoImage);

const MARK_MS = 450;
const TEXT_DELAY_MS = 380;
const TEXT_MS = 380;
const FADE_MS = 280;

/**
 * The in-app launch screen (Expo Go never shows the native splash image): the Lebane isotype
 * scales and fades in, then "Lenapay" and "by Lebane", and once the app is `ready` (and the
 * minimum time passed) it crossfades into the app underneath. With Reduce Motion it shows the
 * final frame and leaves as soon as the app is ready. `onShown` fires when it is on screen, so
 * the native splash can hide without a blank frame in between.
 */
export function LaunchScreen({ ready, onShown, onDone }: { ready: boolean; onShown: () => void; onDone: () => void }) {
  const { t } = useTranslation();
  const reduceMotion = useReducedMotion();
  const [startedAt] = useState(() => Date.now());
  const mark = useSharedValue(reduceMotion ? 1 : 0);
  const text = useSharedValue(reduceMotion ? 1 : 0);
  const cover = useSharedValue(1);

  useEffect(() => {
    if (reduceMotion) return;
    mark.set(withTiming(1, { duration: MARK_MS, easing: Easing.out(Easing.back(1.6)) }));
    text.set(withDelay(TEXT_DELAY_MS, withTiming(1, { duration: TEXT_MS, easing: Easing.out(Easing.cubic) })));
  }, [mark, text, reduceMotion]);

  useEffect(() => {
    if (!ready) return undefined;
    const hold = launchHoldMs({ elapsedMs: Date.now() - startedAt, reduceMotion });
    const fade = reduceMotion ? 0 : FADE_MS;
    const startFade = setTimeout(() => cover.set(withTiming(0, { duration: fade })), hold);
    const finish = setTimeout(onDone, hold + fade);
    return () => {
      clearTimeout(startFade);
      clearTimeout(finish);
    };
  }, [ready, startedAt, reduceMotion, cover, onDone]);

  const coverStyle = useAnimatedStyle(() => ({ opacity: cover.get() }));
  const markStyle = useAnimatedStyle(() => ({ opacity: mark.get(), transform: [{ scale: 0.6 + 0.4 * mark.get() }] }));
  const textStyle = useAnimatedStyle(() => ({ opacity: text.get(), transform: [{ translateY: 10 * (1 - text.get()) }] }));

  return (
    // Reanimated views take `style` only; the token classes live on the plain views inside.
    <Animated.View nativeID="launch-screen" style={[StyleSheet.absoluteFill, coverStyle]} pointerEvents={ready ? 'none' : 'auto'} onLayout={onShown}>
      <View className="flex-1 items-center justify-center gap-5 bg-bg-page" accessible accessibilityLabel={t('brand.byLebaneLabel')}>
        <Animated.View style={markStyle}>
          <Image source={LEBANE_ISOTYPE} className="h-24 w-24" contentFit="contain" cachePolicy="memory" />
        </Animated.View>
        <Animated.View style={textStyle}>
          <View className="items-center gap-1.5">
            <AppText variant="balance" tone="brand">
              {t('brand.name')}
            </AppText>
            <View className="flex-row items-center gap-1.5">
              <AppText variant="caption" tone="muted">
                {t('brand.by')}
              </AppText>
              <View className="dark:hidden">
                <Image source={LEBANE_WORDMARK_ON_LIGHT} className="h-3.5 w-[60px]" contentFit="contain" cachePolicy="memory" />
              </View>
              <View className="hidden dark:flex">
                <Image source={LEBANE_WORDMARK_ON_DARK} className="h-3.5 w-[60px]" contentFit="contain" cachePolicy="memory" />
              </View>
            </View>
          </View>
        </Animated.View>
      </View>
    </Animated.View>
  );
}
