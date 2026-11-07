import { memo } from 'react';
import { View } from 'react-native';

import type { Payer } from '@/shared/api/types';
import { formatCuit, formatDni } from '@/shared/helpers/cuit';
import { AppText } from '@/shared/ui/AppText';
import { PressableCard } from '@/shared/ui/Card';
import { CurrencyChip } from '@/shared/ui/Chip';
import { Icon } from '@/shared/ui/Icon';
import { Avatar } from '@/shared/ui/Rows';

export const PayerRow = memo(function PayerRow({ payer, onPress }: { payer: Payer; onPress: (payer: Payer) => void }) {
  const doc = payer.docType === 'CUIT' ? formatCuit(payer.docNumber) : formatDni(payer.docNumber);
  return (
    <PressableCard onPress={() => onPress(payer)} accessibilityLabel={payer.name} className="mb-3 flex-row items-center gap-3">
      <Avatar name={payer.name} />
      <View className="flex-1 gap-1">
        <AppText variant="bodyStrong" numberOfLines={1}>
          {payer.name}
        </AppText>
        <AppText variant="monoSmall" tone="muted">
          {`${payer.docType} ${doc}`}
        </AppText>
        <View className="flex-row flex-wrap items-center gap-2">
          <AppText variant="caption" tone="secondary" numberOfLines={1}>
            {payer.accountName}
          </AppText>
          {payer.cvus.map((cvu) => (
            <CurrencyChip key={cvu.id} currency={cvu.currency} />
          ))}
        </View>
      </View>
      <Icon name="chevron-forward" size={18} tone="text-muted" />
    </PressableCard>
  );
});
