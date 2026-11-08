import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { AppText } from '@/shared/ui/AppText';
import { Card } from '@/shared/ui/Card';
import { Icon } from '@/shared/ui/Icon';

import type { ChecklistItem } from './kycChecklist';

export function Checklist({ items }: { items: ChecklistItem[] }) {
  const { t } = useTranslation();
  return (
    <Card className="gap-2">
      <AppText variant="label" tone="muted">
        {t('kyc.checklist')}
      </AppText>
      {items.map((item) => (
        <View key={item.key} className="flex-row items-center gap-2" accessible accessibilityLabel={`${t(item.key)}: ${item.done ? t('kyc.done') : t('kyc.pending')}`}>
          <Icon name={item.done ? 'checkmark-circle' : 'ellipse-outline'} size={18} tone={item.done ? 'success' : 'text-muted'} />
          <AppText variant="label" tone={item.done ? 'primary' : 'secondary'}>
            {t(item.key)}
          </AppText>
        </View>
      ))}
    </Card>
  );
}
