import { LinearGradient } from 'expo-linear-gradient';
import type { ReactNode } from 'react';
import { StyleSheet, View } from 'react-native';

import { elevation } from './elevation';
import { useThemeColor } from './Icon';

/**
 * The hero surface (balance card): primary gradient, glossy sheen and a coloured glow. Text on
 * it uses the `onPrimary` tone. The shadow lives on the outer view and the clipping on the
 * inner one, because a view that clips its children also clips its own shadow on iOS.
 */
export function HeroSurface({ children }: { children: ReactNode }) {
  const from = useThemeColor('hero-from');
  const to = useThemeColor('hero-to');
  return (
    <View className={`rounded-card ${elevation('glow')}`}>
      <View className="overflow-hidden rounded-card border border-hero-from bg-hero-to">
        {from && to ? <LinearGradient colors={[from, to]} start={{ x: 0, y: 0 }} end={{ x: 1, y: 1 }} style={StyleSheet.absoluteFill} /> : null}
        <View className="absolute -right-16 -top-24 h-56 w-56 rounded-full bg-hero-sheen" />
        <View className="absolute -bottom-20 -left-10 h-40 w-40 rounded-full bg-hero-sheen" />
        <View className="gap-4 p-4">{children}</View>
      </View>
    </View>
  );
}
