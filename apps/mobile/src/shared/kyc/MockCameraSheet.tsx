import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Icon } from '@/shared/ui/Icon';
import { Sheet } from '@/shared/ui/Sheet';

/**
 * Simulated selfie camera: the face frame and the capture button of the real flow, without
 * opening the camera (no permission is requested until the real KYC vendor is wired).
 */
export function MockCameraSheet({ visible, onClose, onCapture, busy }: { visible: boolean; onClose: () => void; onCapture: () => void; busy: boolean }) {
  const { t } = useTranslation();
  return (
    <Sheet visible={visible} title={t('kyc.selfieTitle')} onClose={onClose}>
      <View className="items-center gap-4">
        <View className="h-72 w-full items-center justify-center rounded-card bg-text-primary">
          <View className="h-56 w-44 items-center justify-center rounded-full border-4 border-dashed border-primary">
            <Icon name="person" size={96} tone="text-muted" />
          </View>
        </View>
        <AppText tone="secondary" className="text-center">
          {t('kyc.selfieBody')}
        </AppText>
        <Button label={t('kyc.capture')} icon="camera" onPress={onCapture} loading={busy} />
      </View>
    </Sheet>
  );
}
