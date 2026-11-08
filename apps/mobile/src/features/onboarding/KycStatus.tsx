import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { formatDateTime } from '@/shared/i18n/dates';
import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { Icon } from '@/shared/ui/Icon';
import { Notice } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { ErrorState, SkeletonList } from '@/shared/ui/States';

import { useKycStatus } from './hooks/useKycStatus';

/** KYC status timeline; polls while the provider decides (R-KYC-1). */
export function KycStatus() {
  const { t } = useTranslation();
  const screen = useKycStatus();
  const data = screen.kyc.data;
  const status = data?.status;

  return (
    <Screen
      title={t('kyc.statusTitle')}
      back
      footer={
        status === 'approved' ? (
          <Button label={t('kyc.goHome')} onPress={screen.goHome} />
        ) : status === 'rejected' || status === 'draft' ? (
          <Button label={t('kyc.fix')} onPress={screen.fix} />
        ) : undefined
      }
    >
      {screen.kyc.isLoading ? <SkeletonList rows={3} /> : null}
      {screen.kyc.isError ? <ErrorState error={screen.kyc.error} onRetry={() => void screen.kyc.refetch()} /> : null}
      {data ? (
        <>
          <Card className="gap-4">
            {screen.timeline.map((step, index) => {
              const at = step.at;
              const tone = step.status === 'rejected' ? 'danger' : step.state === 'upcoming' ? 'text-muted' : step.state === 'current' ? 'warning' : 'success';
              return (
                <View key={step.status} className="flex-row gap-3">
                  <View className="items-center">
                    <Icon name={step.state === 'done' ? 'checkmark-circle' : step.state === 'current' ? 'time' : 'ellipse-outline'} size={24} tone={tone} />
                    {index < screen.timeline.length - 1 ? <View className="mt-1 h-6 w-0.5 bg-border-default" /> : null}
                  </View>
                  <View className="flex-1 gap-0.5">
                    <AppText variant="bodyStrong" tone={step.state === 'upcoming' ? 'muted' : 'primary'}>
                      {t(`kyc.timeline.${step.status}`)}
                    </AppText>
                    {at ? (
                      <AppText variant="caption" tone="muted">
                        {formatDateTime(at)}
                      </AppText>
                    ) : null}
                  </View>
                </View>
              );
            })}
          </Card>
          {status === 'submitted' ? <Notice tone="info" icon="hourglass-outline" title={t('kyc.reviewing')} body={t('kyc.reviewingBody')} /> : null}
          {status === 'approved' ? <Notice tone="success" icon="checkmark-circle-outline" title={t('kyc.approvedTitle')} body={t('kyc.approvedBody')} /> : null}
          {status === 'rejected' ? <Notice tone="danger" icon="close-circle-outline" title={t('kyc.rejectedTitle')} body={data.decisionReason ?? ''} /> : null}
        </>
      ) : null}
    </Screen>
  );
}
