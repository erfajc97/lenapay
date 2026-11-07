import type { ReactNode } from 'react';
import { Pressable, View } from 'react-native';

/** Surface card: subtle border, radius 12, no shadow (brand.md). */
export function Card({ children, className }: { children: ReactNode; className?: string }) {
  return <View className={`rounded-card border border-border-subtle bg-bg-surface p-4 ${className ?? ''}`}>{children}</View>;
}

/** A card that is a single tappable target. */
export function PressableCard({ children, onPress, accessibilityLabel, className }: { children: ReactNode; onPress: () => void; accessibilityLabel: string; className?: string }) {
  return (
    <Pressable
      onPress={onPress}
      accessibilityRole="button"
      accessibilityLabel={accessibilityLabel}
      className={`rounded-card border border-border-subtle bg-bg-surface p-4 active:bg-primary-soft ${className ?? ''}`}
    >
      {children}
    </Pressable>
  );
}
