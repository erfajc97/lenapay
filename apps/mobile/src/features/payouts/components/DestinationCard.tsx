import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import type { Destination } from '@/shared/api/types';
import { formatCuit } from '@/shared/helpers/cuit';
import { groupCbu } from '@/shared/helpers/cbu';
import { AppText } from '@/shared/ui/AppText';
import { Card } from '@/shared/ui/Card';
import { Chip } from '@/shared/ui/Chip';
import { Avatar, Field } from '@/shared/ui/Rows';

/** The resolved holder (R-OUT-1): who receives the money, before any amount is typed. */
export function DestinationCard({ destination }: { destination: Destination }) {
  const { t } = useTranslation();
  return (
    <Card className="gap-1">
      <View className="flex-row items-center gap-3 pb-1">
        <Avatar name={destination.holderName} />
        <View className="flex-1 gap-1">
          <AppText variant="bodyStrong">{destination.holderName}</AppText>
          <Chip label={t(`payout.destinationType.${destination.type}`)} tone="brand" />
        </View>
      </View>
      <Field label={t('fields.cuit')} value={formatCuit(destination.holderCuit)} mono />
      <Field label={t('fields.bank')} value={destination.bankName} />
      <Field label={t('fields.cbu')} value={groupCbu(destination.cbu)} mono />
      {destination.alias ? <Field label={t('fields.alias')} value={destination.alias} mono /> : null}
    </Card>
  );
}
