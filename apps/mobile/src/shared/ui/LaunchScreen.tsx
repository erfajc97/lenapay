import { Image as ExpoImage } from 'expo-image';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { StyleSheet, View } from 'react-native';
import Animated, { Easing, useAnimatedStyle, useReducedMotion, useSharedValue, withDelay, withTiming } from 'react-native-reanimated';
import { withUniwind } from 'uniwind';

import { AppText } from './AppText';
import { LEBANE_WORDMARK_ON_DARK, LEBANE_WORDMARK_ON_LIGHT, SPLASH_ICON } from './brandAssets';
import { LAUNCH_MARK_MS, LAUNCH_TEXT_DELAY_MS, LAUNCH_TEXT_MS, launchHoldMs } from './launchTiming';

const Image = withUniwind(ExpoImage);

const FADE_MS = 280;

/**
 * The in-app launch screen. It starts on exactly the native splash frame (same image, 112 dp,
 * centred, same background), the isotype breathes once, then "Lenapay" and "by Lebane" rise in below it, and once the app is `ready` (and the
 * minimum time passed) it crossfades into the app underneath. With Reduce Motion it shows the
 * final frame for a shorter minimum and leaves without animating. `onShown` fires when it is on screen, so
 * the native splash can hide without a blank frame in between.
 */
export function LaunchScreen({ ready, onShown, onDone }: { ready: boolean; onShown: () => void; onDone: () => void }) {
  const { t } = useTranslation();
  const reduceMotion = useReducedMotion();
  const [startedAt] = useState(() => Date.now());
  const mark = useSharedValue(0);
  const text = useSharedValue(reduceMotion ? 1 : 0);
  const cover = useSharedValue(1);

  useEffect(() => {
    if (reduceMotion) return;
    mark.set(withTiming(1, { duration: LAUNCH_MARK_MS, easing: Easing.inOut(Easing.sin) }));
    text.set(withDelay(LAUNCH_TEXT_DELAY_MS, withTiming(1, { duration: LAUNCH_TEXT_MS, easing: Easing.out(Easing.cubic) })));
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
  // Starts at scale 1 (the native frame), breathes to 1.06 and back: never a jump at the handoff.
  const markStyle = useAnimatedStyle(() => ({ transform: [{ scale: 1 + 0.06 * Math.sin(Math.PI * mark.get()) }] }));
  const textStyle = useAnimatedStyle(() => ({ opacity: text.get(), transform: [{ translateY: 10 * (1 - text.get()) }] }));

  return (
    // Reanimated views take `style` only; the token classes live on the plain views inside.
    <Animated.View nativeID="launch-screen" style={[StyleSheet.absoluteFill, coverStyle]} pointerEvents={ready ? 'none' : 'auto'} onLayout={onShown}>
      {/* The native splash colours (#FFFFFF light = bg-surface, #0E1015 dark = bg-page). */}
      <View className="flex-1 bg-bg-surface dark:bg-bg-page" accessible accessibilityLabel={t('brand.byLebaneLabel')}>
        {/* Same image, size (112 dp, contain) and centre as the native splash: the handoff does not move. */}
        <View style={styles.centre} pointerEvents="none">
          <Animated.View style={markStyle}>
            <Image source={SPLASH_ICON} style={styles.mark} contentFit="contain" cachePolicy="memory" />
          </Animated.View>
        </View>
        <Animated.View style={[styles.below, textStyle]} pointerEvents="none">
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

/** Native splash geometry (app.json `imageWidth`): the mark is centred on the full window. */
const SPLASH_IMAGE_DP = 112;
const styles = StyleSheet.create({
  centre: { position: 'absolute', top: 0, right: 0, bottom: 0, left: 0, alignItems: 'center', justifyContent: 'center' },
  mark: { width: SPLASH_IMAGE_DP, height: SPLASH_IMAGE_DP },
  below: { position: 'absolute', left: 0, right: 0, top: '50%', marginTop: SPLASH_IMAGE_DP / 2 + 20, alignItems: 'center' },
});
