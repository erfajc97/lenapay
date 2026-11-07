import { useTranslation } from 'react-i18next';
import { Pressable, View } from 'react-native';

import type { Cvu } from '@/shared/api/types';
import { groupCbu } from '@/shared/helpers/cbu';
import { AppText } from '@/shared/ui/AppText';
import { Card } from '@/shared/ui/Card';
import { Chip, CurrencyChip } from '@/shared/ui/Chip';
import { Icon, type IconName } from '@/shared/ui/Icon';
import { Divider, LinkButton } from '@/shared/ui/Rows';

function IconAction({ icon, label, onPress }: { icon: IconName; label: string; onPress: () => void }) {
  return (
    <Pressable onPress={onPress} accessibilityRole="button" accessibilityLabel={label} className="min-h-11 flex-1 items-center justify-center gap-1 rounded-chip py-2 active:bg-primary-soft">
      <Icon name={icon} size={20} tone="primary" />
      <AppText variant="caption" tone="brand">
        {label}
      </AppText>
    </Pressable>
  );
}

/** One CVU: number and alias in JetBrains Mono, copy, share, QR (R-CVU-2). */
export function CvuCard({
  cvu,
  canEdit,
  onCopy,
  onShare,
  onQr,
  onAlias,
  onDisable,
}: {
  cvu: Cvu;
  canEdit: boolean;
  onCopy: (value: string) => void;
  onShare: (cvu: Cvu) => void;
  onQr: (cvu: Cvu) => void;
  onAlias: (cvu: Cvu) => void;
  onDisable: (cvu: Cvu) => void;
}) {
  const { t } = useTranslation();
  const active = cvu.status === 'active';
  return (
    <Card className={`gap-3 ${active ? '' : 'opacity-60'}`}>
      <View className="flex-row items-center justify-between">
        <CurrencyChip currency={cvu.currency} />
        <Chip label={t(`payers.cvuStatus.${cvu.status}`)} tone={active ? 'success' : 'neutral'} />
      </View>
      <View className="gap-1">
        <AppText variant="caption" tone="muted">
          {t('fields.cvu')}
        </AppText>
        <AppText variant="mono" selectable accessibilityLabel={t('payers.cvuA11y', { value: cvu.cvu.split('').join(' ') })}>
          {groupCbu(cvu.cvu)}
        </AppText>
      </View>
      <View className="gap-1">
        <AppText variant="caption" tone="muted">
          {t('fields.alias')}
        </AppText>
        <AppText variant="mono" selectable>
          {cvu.alias}
        </AppText>
      </View>
      {active ? (
        <>
          <Divider />
          <View className="flex-row">
            <IconAction icon="copy-outline" label={t('payers.copyCvu')} onPress={() => onCopy(cvu.cvu)} />
            <IconAction icon="at-outline" label={t('payers.copyAlias')} onPress={() => onCopy(cvu.alias)} />
            <IconAction icon="share-outline" label={t('common.share')} onPress={() => onShare(cvu)} />
            <IconAction icon="qr-code-outline" label={t('payers.qr')} onPress={() => onQr(cvu)} />
          </View>
          {canEdit ? (
            <View className="flex-row justify-between">
              <LinkButton label={t('payers.editAlias')} onPress={() => onAlias(cvu)} />
              <LinkButton label={t('payers.disable')} onPress={() => onDisable(cvu)} />
            </View>
          ) : null}
        </>
      ) : null}
    </Card>
  );
}
