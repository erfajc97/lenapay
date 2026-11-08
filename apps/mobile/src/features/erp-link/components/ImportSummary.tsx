import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import type { ErpImportSummary } from '@/shared/api/types';
import { AppText } from '@/shared/ui/AppText';
import { Card } from '@/shared/ui/Card';
import { Chip } from '@/shared/ui/Chip';
import { Avatar, Divider, SectionTitle } from '@/shared/ui/Rows';

function Count({ label, value }: { label: string; value: number }) {
  return (
    <View className="min-h-11 flex-row items-center justify-between gap-3 py-1">
      <AppText tone="secondary" className="flex-1">
        {label}
      </AppText>
      <AppText variant="bodyStrong">{String(value)}</AppText>
    </View>
  );
}

/** What the ERP import brought (R-ERP-4): counts, the Lebane users invited with their role, and who was left out. */
export function ImportSummary({ summary }: { summary: ErpImportSummary }) {
  const { t } = useTranslation();
  return (
    <View className="gap-3">
      <SectionTitle title={t('erp.summaryTitle')} />
      <Card className="py-1">
        <Count label={t('erp.summaryProjects')} value={summary.projects} />
        <Divider />
        <Count label={t('erp.summaryBuyers')} value={summary.buyers} />
      </Card>
      <SectionTitle title={t('erp.invitedTitle')} />
      <Card className="gap-1 py-2">
        <AppText variant="caption" tone="muted">
          {summary.invitedUsers.length > 0 ? t('erp.invitedBody') : t('erp.noneInvited')}
        </AppText>
        {summary.invitedUsers.map((user) => (
          <View key={user.email}>
            <Divider />
            <View className="min-h-12 flex-row items-center gap-3 py-1" accessible accessibilityLabel={t('erp.invitedA11y', { email: user.email, role: t(`roles.${user.role}`) })}>
              <Avatar name={user.email} />
              <AppText className="flex-1" numberOfLines={1}>
                {user.email}
              </AppText>
              <View className="justify-center">
                <Chip label={t(`roles.${user.role}`)} tone="brand" />
              </View>
            </View>
          </View>
        ))}
        <Divider />
        {summary.usersAlreadyMembers > 0 ? <Count label={t('erp.alreadyMembers')} value={summary.usersAlreadyMembers} /> : null}
        <Count label={t('erp.withoutAccess')} value={summary.usersWithoutPaymentsAccess} />
      </Card>
    </View>
  );
}
