import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { MEMBER_ROLES } from '@/shared/api/types';
import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { Chip, FilterChip } from '@/shared/ui/Chip';
import { Avatar, Divider, IconBadge, ListRow, Notice } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { Sheet } from '@/shared/ui/Sheet';
import { EmptyState, ErrorState, SkeletonList } from '@/shared/ui/States';
import { TextField } from '@/shared/ui/TextField';

import { AccountPermissionsProposal } from './components/AccountPermissionsProposal';
import { TransferOwnershipSheet } from './components/TransferOwnershipSheet';
import { useTeam } from './hooks/useTeam';

/** Members, roles, invites and ownership (R-ORG-2, R-ORG-3). */
export function Team() {
  const { t } = useTranslation();
  const team = useTeam();
  return (
    <Screen
      title={t('team.title')}
      back
      onRefresh={team.members.refetch}
      footer={team.canManage ? <Button label={t('team.invite')} icon="person-add-outline" onPress={team.openSheet} /> : undefined}
    >
      {!team.canManage ? <Notice tone="info" icon="information-circle-outline" title={t('team.readOnly')} /> : null}
      {team.members.isLoading ? <SkeletonList rows={4} /> : null}
      {team.members.isError ? <ErrorState error={team.members.error} onRetry={() => void team.members.refetch()} /> : null}
      {team.members.isSuccess && team.members.data.length === 0 ? <EmptyState icon="people-outline" title={t('team.empty')} /> : null}
      {team.members.data ? (
        <Card className="py-1">
          {team.members.data.map((member, index) => (
            <View key={member.userId}>
              {index > 0 ? <Divider /> : null}
              <ListRow
                title={team.nameOf(member)}
                subtitle={member.status === 'invited' ? t('team.pendingInvite') : member.fullName ? member.email : null}
                leading={<Avatar name={team.nameOf(member)} />}
                trailing={
                  <View className="items-end gap-1">
                    <Chip label={t(`roles.${member.role}`)} tone={member.status === 'invited' ? 'warning' : member.role === 'owner' ? 'success' : 'brand'} />
                    {team.isMe(member) ? <Chip label={t('team.you')} tone="neutral" /> : null}
                  </View>
                }
                onPress={team.canEdit(member) ? () => team.openMember(member) : undefined}
              />
            </View>
          ))}
        </Card>
      ) : null}
      {team.ladderNote ? (
        <AppText variant="caption" tone="muted">
          {t(team.ladderNote)}
        </AppText>
      ) : null}
      {team.canTransfer ? (
        <Card className="py-1">
          <ListRow title={t('team.transfer')} subtitle={t('team.transferRowBody')} leading={<IconBadge name="key-outline" />} onPress={team.transfer.openSheet} />
        </Card>
      ) : null}
      <View className="gap-2">
        {MEMBER_ROLES.map((role) => (
          <View key={role} className="flex-row gap-2">
            <AppText variant="label" className="w-28">
              {t(`roles.${role}`)}
            </AppText>
            <AppText variant="caption" tone="muted" className="flex-1">
              {t(`roleHelp.${role}`)}
            </AppText>
          </View>
        ))}
      </View>

      <Sheet visible={team.editing !== null} title={t('team.memberTitle')} onClose={team.closeMember}>
        <AppText variant="bodyStrong">{team.editing ? team.nameOf(team.editing) : ''}</AppText>
        <AppText variant="label" tone="secondary">
          {t('fields.role')}
        </AppText>
        <View className="flex-row flex-wrap gap-2">
          {team.grantable.map((role) => (
            <FilterChip key={role} label={t(`roles.${role}`)} selected={team.editing?.role === role} onPress={() => team.changeRole(role)} />
          ))}
        </View>
        <Button
          label={team.confirmRemove ? t('team.confirmRemove') : t('team.remove')}
          variant="danger"
          icon="person-remove-outline"
          onPress={team.remove}
          loading={team.savingMember}
        />
        {team.accounts.length > 0 ? <AccountPermissionsProposal accounts={team.accounts} /> : null}
      </Sheet>

      <Sheet visible={team.sheetOpen} title={t('team.invite')} onClose={team.closeSheet}>
        <TextField
          label={t('fields.email')}
          value={team.email}
          onChangeText={team.setEmail}
          error={team.emailError ? t(team.emailError) : null}
          keyboardType="email-address"
          autoCapitalize="none"
          autoComplete="email"
          textContentType="emailAddress"
          returnKeyType="done"
          autoFocus
        />
        <AppText variant="label" tone="secondary">
          {t('fields.role')}
        </AppText>
        <View className="flex-row flex-wrap gap-2">
          {team.grantable.map((role) => (
            <FilterChip key={role} label={t(`roles.${role}`)} selected={team.role === role} onPress={() => team.setRole(role)} />
          ))}
        </View>
        <AppText variant="caption" tone="muted">
          {t('team.expires')}
        </AppText>
        <Button label={t('team.send')} onPress={team.submit} loading={team.inviting} />
      </Sheet>

      <TransferOwnershipSheet
        visible={team.transfer.open}
        orgName={team.org?.legalName ?? ''}
        candidates={team.transfer.candidates}
        target={team.transfer.target}
        onPick={team.transfer.pick}
        onConfirm={team.transfer.confirm}
        onClose={team.transfer.close}
        pending={team.transfer.pending}
      />
    </Screen>
  );
}
