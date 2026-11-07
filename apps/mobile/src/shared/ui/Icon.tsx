import Ionicons from '@expo/vector-icons/Ionicons';
import type { ComponentProps } from 'react';
import { useCSSVariable } from 'uniwind';

export type IconName = ComponentProps<typeof Ionicons>['name'];
export type IconTone = 'text-primary' | 'text-secondary' | 'text-muted' | 'primary' | 'success' | 'warning' | 'danger' | 'on-primary' | 'accent';

/** Icons take their colour from the theme tokens, never a literal (M9). Decorative by default. */
export function Icon({ name, size = 20, tone = 'text-secondary' }: { name: IconName; size?: number; tone?: IconTone }) {
  const color = useCSSVariable(`--color-${tone}`);
  return <Ionicons name={name} size={size} color={typeof color === 'string' ? color : undefined} accessibilityElementsHidden importantForAccessibility="no" />;
}

/** A theme colour for third-party props that only take a colour string. */
export function useThemeColor(token: string): string | undefined {
  const value = useCSSVariable(`--color-${token}`);
  return typeof value === 'string' ? value : undefined;
}
