import type { ReactNode } from 'react';
import { Pressable, View } from 'react-native';

import { elevation, pressedScale } from './elevation';

const SURFACE = 'rounded-card border border-border-subtle bg-bg-surface p-4';

/** Surface card: subtle border, radius 16, soft layered shadow (brand.md "Elevation": md). */
export function Card({ children, className }: { children: ReactNode; className?: string }) {
  return <View className={`${SURFACE} ${elevation('md')} ${className ?? ''}`}>{children}</View>;
}

/** A card that is a single tappable target: it shrinks and sinks while pressed. */
export function PressableCard({ children, onPress, accessibilityLabel, className }: { children: ReactNode; onPress: () => void; accessibilityLabel: string; className?: string }) {
  return (
    <Pressable
      onPress={onPress}
      accessibilityRole="button"
      accessibilityLabel={accessibilityLabel}
      style={pressedScale}
      className={`${SURFACE} ${elevation('md', { pressable: true })} active:bg-primary-soft ${className ?? ''}`}
    >
      {children}
    </Pressable>
  );
}
