import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { ROLES } from '@/shared/api/types';
import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { Chip, FilterChip } from '@/shared/ui/Chip';
import { Avatar, Divider, ListRow, Notice } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { Sheet } from '@/shared/ui/Sheet';
import { EmptyState, ErrorState, SkeletonList } from '@/shared/ui/States';
import { TextField } from '@/shared/ui/TextField';

import { AccountPermissionsProposal } from './components/AccountPermissionsProposal';
import { useTeam } from './hooks/useTeam';

/** Members, roles and invites (R-ORG-2). */
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
                title={member.fullName ?? member.email}
                subtitle={member.status === 'invited' ? t('team.pendingInvite') : member.fullName ? member.email : null}
                leading={<Avatar name={member.fullName ?? member.email} />}
                trailing={<Chip label={t(`roles.${member.role}`)} tone={member.status === 'invited' ? 'warning' : 'brand'} />}
                onPress={team.canEdit(member) ? () => team.openMember(member) : undefined}
              />
            </View>
          ))}
        </Card>
      ) : null}
      <View className="gap-2">
        {ROLES.map((role) => (
          <View key={role} className="flex-row gap-2">
            <AppText variant="label" className="w-24">
              {t(`roles.${role}`)}
            </AppText>
            <AppText variant="caption" tone="muted" className="flex-1">
              {t(`roleHelp.${role}`)}
            </AppText>
          </View>
        ))}
      </View>

      <Sheet visible={team.editing !== null} title={t('team.memberTitle')} onClose={team.closeMember}>
        <AppText variant="bodyStrong">{team.editing?.fullName ?? team.editing?.email ?? ''}</AppText>
        <AppText variant="label" tone="secondary">
          {t('fields.role')}
        </AppText>
        <View className="flex-row flex-wrap gap-2">
          {ROLES.map((role) => (
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
          {ROLES.map((role) => (
            <FilterChip key={role} label={t(`roles.${role}`)} selected={team.role === role} onPress={() => team.setRole(role)} />
          ))}
        </View>
        <AppText variant="caption" tone="muted">
          {t('team.expires')}
        </AppText>
        <Button label={t('team.send')} onPress={team.submit} loading={team.inviting} />
      </Sheet>
    </Screen>
  );
}
