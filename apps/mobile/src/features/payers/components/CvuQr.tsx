import { View } from 'react-native';
import QRCode from 'react-native-qrcode-svg';

import { useThemeColor } from '@/shared/ui/Icon';

/** QR of the CVU with theme colours (dark modules on the surface colour). */
export function CvuQr({ value, label }: { value: string; label: string }) {
  const foreground = useThemeColor('text-primary');
  const background = useThemeColor('bg-surface');
  return (
    <View className="items-center rounded-card border border-border-subtle bg-bg-surface p-4" accessible accessibilityRole="image" accessibilityLabel={label}>
      <QRCode value={value} size={208} color={foreground} backgroundColor={background} ecl="M" />
    </View>
  );
}
