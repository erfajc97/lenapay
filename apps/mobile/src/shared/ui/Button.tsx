import { ActivityIndicator, Pressable, View } from 'react-native';

import { AppText, type TextTone } from './AppText';
import { Icon, type IconName, type IconTone } from './Icon';

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger';

const CONTAINER: Record<ButtonVariant, string> = {
  primary: 'bg-primary active:bg-primary-active',
  secondary: 'bg-bg-surface border border-border-default active:bg-primary-soft',
  ghost: 'bg-transparent active:bg-primary-soft',
  danger: 'bg-danger-bg active:opacity-80',
};
const LABEL: Record<ButtonVariant, TextTone> = { primary: 'onPrimary', secondary: 'primary', ghost: 'brand', danger: 'danger' };
const ICON: Record<ButtonVariant, IconTone> = { primary: 'on-primary', secondary: 'text-primary', ghost: 'primary', danger: 'danger' };

interface ButtonProps {
  label: string;
  onPress: () => void;
  variant?: ButtonVariant;
  icon?: IconName;
  loading?: boolean;
  disabled?: boolean;
  compact?: boolean;
  /** Extra screen-reader context when the label alone is ambiguous. */
  accessibilityHint?: string;
}

/** Pill button (radius 20). Disabled while its mutation runs (M6); state exposed to a11y (M7). */
export function Button({ label, onPress, variant = 'primary', icon, loading, disabled, compact, accessibilityHint }: ButtonProps) {
  const inactive = Boolean(disabled || loading);
  return (
    <Pressable
      onPress={onPress}
      disabled={inactive}
      accessibilityRole="button"
      accessibilityLabel={label}
      accessibilityHint={accessibilityHint}
      accessibilityState={{ disabled: inactive, busy: Boolean(loading) }}
      className={`flex-row items-center justify-center gap-2 rounded-button ${compact ? 'min-h-11 px-4' : 'min-h-13 px-5'} ${CONTAINER[variant]} ${inactive ? 'opacity-50' : ''}`}
    >
      {loading ? (
        <ActivityIndicator colorClassName={variant === 'primary' ? 'accent-on-primary' : 'accent-primary'} />
      ) : (
        <View className="flex-row items-center gap-2">
          {icon ? <Icon name={icon} size={18} tone={ICON[variant]} /> : null}
          <AppText variant="bodyStrong" tone={LABEL[variant]}>
            {label}
          </AppText>
        </View>
      )}
    </Pressable>
  );
}
