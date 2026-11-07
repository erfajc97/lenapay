import { TextInput, type TextInputProps, View } from 'react-native';

import { AppText } from './AppText';
import { FOCUS_RING } from './elevation';

export interface TextFieldProps extends Omit<TextInputProps, 'className'> {
  label: string;
  error?: string | null;
  hint?: string | null;
  mono?: boolean;
  /** Large amount entry. */
  large?: boolean;
}

/** Labelled input, radius 4, primary focus ring with a soft glow. Callers set keyboardType, autoComplete and returnKeyType (M10). */
export function TextField({ label, error, hint, mono, large, editable = true, ...props }: TextFieldProps) {
  const font = mono ? 'font-mono text-base' : large ? 'font-sans-bold text-2xl' : 'font-sans text-base';
  return (
    <View className="gap-1.5">
      <AppText variant="label" tone="secondary">
        {label}
      </AppText>
      <TextInput
        accessibilityLabel={label}
        accessibilityHint={error ?? hint ?? undefined}
        editable={editable}
        placeholderTextColorClassName="accent-text-disabled"
        selectionColorClassName="accent-primary"
        cursorColorClassName="accent-primary"
        className={`min-h-12 rounded-input border bg-bg-surface px-3 py-2.5 text-text-primary ${font} ${error ? 'border-danger' : `border-border-default ${FOCUS_RING}`} ${editable ? '' : 'opacity-60'}`}
        {...props}
      />
      {error ? (
        <AppText variant="caption" tone="danger" accessibilityLiveRegion="polite">
          {error}
        </AppText>
      ) : hint ? (
        <AppText variant="caption" tone="muted">
          {hint}
        </AppText>
      ) : null}
    </View>
  );
}
