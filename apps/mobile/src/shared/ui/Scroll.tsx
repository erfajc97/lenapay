import { ScrollView, type ScrollViewProps } from 'react-native';

/**
 * The ONE vertical scroll primitive (ME9). Insets come from the screen wrapper, so iOS must
 * not add its own; bounce only with pull-to-refresh; taps reach buttons with the keyboard
 * open. Caller props win.
 */
export function Scroll(props: ScrollViewProps & { className?: string; contentContainerClassName?: string }) {
  return (
    <ScrollView
      contentInsetAdjustmentBehavior="never"
      alwaysBounceVertical={Boolean(props.refreshControl)}
      keyboardShouldPersistTaps="handled"
      showsVerticalScrollIndicator={false}
      {...props}
    />
  );
}
