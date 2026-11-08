import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import type { Member } from '@/shared/api/types';
import { displayName } from '@/shared/helpers/people';
import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Chip } from '@/shared/ui/Chip';
import { Icon } from '@/shared/ui/Icon';
import { Avatar, Divider, ListRow, Notice } from '@/shared/ui/Rows';
import { Sheet } from '@/shared/ui/Sheet';

interface Props {
  visible: boolean;
  orgName: string;
  candidates: Member[];
  target: Member | null;
  onPick: (member: Member) => void;
  onConfirm: () => void;
  onClose: () => void;
  pending: boolean;
}

/** "Transferir la propiedad" (R-ORG-3): pick a member, read what changes, confirm with step-up. */
export function TransferOwnershipSheet({ visible, orgName, candidates, target, onPick, onConfirm, onClose, pending }: Props) {
  const { t } = useTranslation();
  const targetName = target ? displayName(target) : '';
  return (
    <Sheet visible={visible} title={t('team.transfer')} onClose={onClose}>
      {candidates.length === 0 ? (
        <Notice tone="info" icon="information-circle-outline" title={t('team.transferEmpty')} />
      ) : (
        <>
          <AppText tone="secondary">{t('team.transferBody', { org: orgName })}</AppText>
          <View accessibilityRole="radiogroup" accessibilityLabel={t('team.transfer')}>
            {candidates.map((member, index) => (
              <View key={member.userId}>
                {index > 0 ? <Divider /> : null}
                <ListRow
                  title={displayName(member)}
                  subtitle={member.fullName ? member.email : null}
                  leading={<Avatar name={displayName(member)} />}
                  trailing={
                    <View className="flex-row items-center gap-2">
                      <Chip label={t(`roles.${member.role}`)} tone="brand" />
                      {target?.userId === member.userId ? <Icon name="checkmark" size={20} tone="primary" /> : null}
                    </View>
                  }
                  onPress={() => onPick(member)}
                  chevron={false}
                  selected={target?.userId === member.userId}
                />
              </View>
            ))}
          </View>
          {target ? (
            <Notice tone="warning" icon="warning-outline" title={t('team.transferWarningTitle')} body={t('team.transferWarningBody', { name: targetName })} />
          ) : (
            <AppText variant="caption" tone="muted">
              {t('team.transferPick')}
            </AppText>
          )}
          <Button
            label={target ? t('team.transferConfirm', { name: targetName }) : t('team.transfer')}
            variant="danger"
            icon="finger-print"
            onPress={onConfirm}
            loading={pending}
            disabled={!target}
          />
        </>
      )}
    </Sheet>
  );
}
