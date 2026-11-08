import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import type { AccountCvu } from '@/shared/api/types';
import { groupCbu } from '@/shared/helpers/cbu';
import { AppText } from '@/shared/ui/AppText';
import { Card } from '@/shared/ui/Card';
import { CurrencyChip } from '@/shared/ui/Chip';
import { IconAction } from '@/shared/ui/IconAction';
import { Divider, LinkButton, SectionTitle } from '@/shared/ui/Rows';

/** "Mi CVU" (R-CVU-4): the wallet's CVU and alias per currency, copy, share, QR and alias edit. */
export function MyCvuCard({
  cvus,
  onCopy,
  onShare,
  onQr,
  onEditAlias,
}: {
  cvus: AccountCvu[];
  onCopy: (value: string) => void;
  onShare: (cvu: AccountCvu) => void;
  onQr: (cvu: AccountCvu) => void;
  onEditAlias: (cvu: AccountCvu) => void;
}) {
  const { t } = useTranslation();
  return (
    <View className="gap-2">
      <SectionTitle title={t('home.myCvu.title')} />
      <Card className="gap-3">
        {cvus.map((cvu, index) => (
          <View key={cvu.id} className="gap-2">
            {index > 0 ? <Divider /> : null}
            <View className="flex-row items-center justify-between">
              <CurrencyChip currency={cvu.currency} />
              <LinkButton label={t('payers.editAlias')} onPress={() => onEditAlias(cvu)} />
            </View>
            <View className="gap-0.5">
              <AppText variant="caption" tone="muted">
                {t('fields.cvu')}
              </AppText>
              <AppText variant="mono" selectable accessibilityLabel={t('payers.cvuA11y', { value: cvu.cvu.split('').join(' ') })}>
                {groupCbu(cvu.cvu)}
              </AppText>
            </View>
            <View className="gap-0.5">
              <AppText variant="caption" tone="muted">
                {t('fields.alias')}
              </AppText>
              <AppText variant="mono" selectable>
                {cvu.alias ?? t('home.myCvu.noAlias')}
              </AppText>
            </View>
            <View className="flex-row">
              <IconAction icon="copy-outline" label={t('payers.copyCvu')} onPress={() => onCopy(cvu.cvu)} />
              {cvu.alias ? <IconAction icon="at-outline" label={t('payers.copyAlias')} onPress={() => onCopy(cvu.alias ?? '')} /> : null}
              <IconAction icon="share-outline" label={t('common.share')} onPress={() => onShare(cvu)} />
              <IconAction icon="qr-code-outline" label={t('payers.qr')} onPress={() => onQr(cvu)} />
            </View>
          </View>
        ))}
      </Card>
    </View>
  );
}
