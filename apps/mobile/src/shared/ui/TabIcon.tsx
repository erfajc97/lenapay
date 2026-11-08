import Ionicons from '@expo/vector-icons/Ionicons';
import { useEffect } from 'react';
import { type ColorValue, StyleSheet, View } from 'react-native';
import Animated, { ReduceMotion, useAnimatedStyle, useSharedValue, withSpring } from 'react-native-reanimated';

import { elevation, SURFACE_TONE } from './elevation';
import { type IconName, useThemeColor } from './Icon';
import { tabPop } from './tabPop';

const SPRING = { damping: 14, stiffness: 220, mass: 0.7, reduceMotion: ReduceMotion.System } as const;

/**
 * Tab bar icon. The selected tab pops: it rises and grows on a spring (UI thread, Reanimated;
 * with Reduce Motion it jumps), sits in a primary circle with the glow and shows the filled
 * icon; the others stay flat and muted. The label (navigator) also turns primary, so the
 * selection never rests on colour alone.
 */
export function TabIcon({ name, color, focused }: { name: IconName; color: ColorValue; focused: boolean }) {
  const onPrimary = useThemeColor('on-primary');
  const progress = useSharedValue(focused ? 1 : 0);
  useEffect(() => {
    progress.set(withSpring(focused ? 1 : 0, SPRING));
  }, [focused, progress]);

  const popStyle = useAnimatedStyle(() => {
    const pop = tabPop(progress.get());
    return { transform: [{ translateY: pop.translateY }, { scale: pop.scale }] };
  });
  const circleStyle = useAnimatedStyle(() => ({ opacity: progress.get() }));

  const outline = `${String(name)}-outline` as IconName;
  const iconColor = focused ? onPrimary : typeof color === 'string' ? color : undefined;
  return (
    // Reanimated views take `style` only; the token classes live on the plain views inside.
    <Animated.View style={[styles.slot, popStyle]}>
      <Animated.View style={[styles.circle, circleStyle]}>
        <View className={`h-11 w-11 rounded-full bg-primary ${elevation('glow')}`} />
      </Animated.View>
      <Ionicons name={focused ? name : outline} size={26} color={iconColor} />
    </Animated.View>
  );
}

// The slot keeps the navigator's icon height (36) so the label stays in place; the 44 dp circle
// overflows it evenly above and below.
const styles = StyleSheet.create({
  slot: { width: 44, height: 36, alignItems: 'center', justifyContent: 'center', overflow: 'visible' },
  circle: { position: 'absolute', top: -4, left: 0, width: 44, height: 44 },
});

/** Floating tab bar surface: rounded top corners over the page colour, lifted by a shadow. */
export function TabBarBackground() {
  return (
    <View className="absolute inset-0 bg-bg-page">
      <View className={`absolute inset-0 rounded-t-sheet border-x border-t border-border-subtle ${SURFACE_TONE.bar} ${elevation('bar')}`} />
    </View>
  );
}
