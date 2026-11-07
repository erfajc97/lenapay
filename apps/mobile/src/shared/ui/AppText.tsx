import { Text, type TextProps } from 'react-native';

/** Typography from brand.md: Plus Jakarta Sans (400/600/700) and JetBrains Mono for keys. */
export type TextVariant = 'balance' | 'title' | 'heading' | 'subheading' | 'body' | 'bodyStrong' | 'label' | 'caption' | 'mono' | 'monoSmall';
export type TextTone = 'primary' | 'secondary' | 'muted' | 'disabled' | 'brand' | 'success' | 'warning' | 'danger' | 'onPrimary' | 'onCurrency';

const VARIANTS: Record<TextVariant, string> = {
  balance: 'font-sans-bold text-balance',
  title: 'font-sans-semibold text-2xl',
  heading: 'font-sans-semibold text-xl',
  subheading: 'font-sans-semibold text-lg',
  body: 'font-sans text-base',
  bodyStrong: 'font-sans-semibold text-base',
  label: 'font-sans-medium text-sm',
  caption: 'font-sans text-xs',
  mono: 'font-mono text-base',
  monoSmall: 'font-mono text-sm',
};

const TONES: Record<TextTone, string> = {
  primary: 'text-text-primary',
  secondary: 'text-text-secondary',
  muted: 'text-text-muted',
  disabled: 'text-text-disabled',
  brand: 'text-primary',
  success: 'text-success',
  warning: 'text-warning',
  danger: 'text-danger',
  onPrimary: 'text-on-primary',
  onCurrency: 'text-on-currency',
};

interface AppTextProps extends TextProps {
  variant?: TextVariant;
  tone?: TextTone;
  className?: string;
}

export function AppText({ variant = 'body', tone = 'primary', className, ...props }: AppTextProps) {
  return <Text {...props} className={`${VARIANTS[variant]} ${TONES[tone]} ${className ?? ''}`} />;
}
