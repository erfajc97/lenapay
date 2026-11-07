import type { ReactNode } from 'react';
import { Pressable, View } from 'react-native';

import { AppText } from './AppText';
import { Icon, type IconName, type IconTone } from './Icon';

/** Round icon badge used at the start of list rows. */
export function IconBadge({ name, tone = 'primary', soft = 'bg-primary-bg' }: { name: IconName; tone?: IconTone; soft?: string }) {
  return (
    <View className={`h-10 w-10 items-center justify-center rounded-full ${soft}`}>
      <Icon name={name} size={20} tone={tone} />
    </View>
  );
}

export function Avatar({ name }: { name: string }) {
  const initials = name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((word) => word[0]?.toUpperCase() ?? '')
    .join('');
  return (
    <View className="h-10 w-10 items-center justify-center rounded-full bg-primary-bg" accessibilityElementsHidden importantForAccessibility="no">
      <AppText variant="label" tone="brand">
        {initials}
      </AppText>
    </View>
  );
}

/** A tappable row: leading badge, title + subtitle, trailing content and chevron. */
export function ListRow({
  title,
  subtitle,
  leading,
  trailing,
  onPress,
  accessibilityLabel,
  chevron = true,
}: {
  title: string;
  subtitle?: string | null;
  leading?: ReactNode;
  trailing?: ReactNode;
  onPress?: () => void;
  accessibilityLabel?: string;
  chevron?: boolean;
}) {
  const content = (
    <>
      {leading}
      <View className="flex-1 gap-0.5">
        <AppText variant="bodyStrong" numberOfLines={1}>
          {title}
        </AppText>
        {subtitle ? (
          <AppText variant="caption" tone="muted" numberOfLines={2}>
            {subtitle}
          </AppText>
        ) : null}
      </View>
      {trailing}
      {onPress && chevron ? <Icon name="chevron-forward" size={18} tone="text-muted" /> : null}
    </>
  );
  if (!onPress) return <View className="min-h-14 flex-row items-center gap-3 py-2">{content}</View>;
  return (
    <Pressable
      onPress={onPress}
      accessibilityRole="button"
      accessibilityLabel={accessibilityLabel ?? title}
      className="min-h-14 flex-row items-center gap-3 rounded-chip py-2 active:bg-primary-soft"
    >
      {content}
    </Pressable>
  );
}

/** Label / value pair for detail screens. Keys (CBU, CVU, CUIT) render in JetBrains Mono. */
export function Field({ label, value, mono, trailing }: { label: string; value: string; mono?: boolean; trailing?: ReactNode }) {
  return (
    <View className="flex-row items-center gap-3 py-2">
      <View className="flex-1 gap-0.5">
        <AppText variant="caption" tone="muted">
          {label}
        </AppText>
        <AppText variant={mono ? 'mono' : 'body'} selectable>
          {value}
        </AppText>
      </View>
      {trailing}
    </View>
  );
}

export function Divider() {
  return <View className="h-px bg-border-subtle" />;
}

export function SectionTitle({ title, action }: { title: string; action?: ReactNode }) {
  return (
    <View className="mt-2 flex-row items-center justify-between">
      <AppText variant="label" tone="muted" accessibilityRole="header" className="uppercase tracking-wide">
        {title}
      </AppText>
      {action}
    </View>
  );
}

/** Inline notice rendered IN the screen (M2: notices that must be seen are not toasts). */
export function Notice({ tone, icon, title, body, action }: { tone: 'info' | 'warning' | 'danger' | 'success'; icon: IconName; title: string; body?: string; action?: ReactNode }) {
  const bg = { info: 'bg-primary-bg', warning: 'bg-warning-bg', danger: 'bg-danger-bg', success: 'bg-success-bg' }[tone];
  const iconTone: IconTone = { info: 'primary', warning: 'warning', danger: 'danger', success: 'success' }[tone] as IconTone;
  return (
    <View className={`flex-row gap-3 rounded-card p-3 ${bg}`}>
      <Icon name={icon} size={20} tone={iconTone} />
      <View className="flex-1 gap-1">
        <AppText variant="bodyStrong">{title}</AppText>
        {body ? (
          <AppText variant="label" tone="secondary">
            {body}
          </AppText>
        ) : null}
        {action}
      </View>
    </View>
  );
}

/** Small text link with a 44 pt touch target. */
export function LinkButton({ label, onPress }: { label: string; onPress: () => void }) {
  return (
    <Pressable onPress={onPress} accessibilityRole="button" accessibilityLabel={label} hitSlop={12} className="min-h-11 justify-center">
      <AppText variant="label" tone="brand">
        {label}
      </AppText>
    </Pressable>
  );
}
