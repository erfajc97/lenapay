import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { AppText } from './AppText';
import { Icon } from './Icon';

/** "Próximamente · propuesta": marks a screen or section that only shows how it would work. */
export function ProposalRibbon({ body }: { body?: string }) {
  const { t } = useTranslation();
  return (
    <View className="gap-1 rounded-card border border-accent bg-warning-bg p-3" accessibilityRole="text">
      <View className="flex-row items-center gap-2">
        <Icon name="sparkles-outline" size={18} tone="warning" />
        <AppText variant="bodyStrong" className="flex-1">
          {t('proposal.ribbon')}
        </AppText>
      </View>
      {body ? (
        <AppText variant="label" tone="secondary">
          {body}
        </AppText>
      ) : null}
    </View>
  );
}
