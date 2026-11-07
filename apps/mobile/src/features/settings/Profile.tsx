import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { useMe } from '@/shared/queries/session';
import { useOrgs } from '@/shared/queries/orgs';
import { AppText } from '@/shared/ui/AppText';
import { Card } from '@/shared/ui/Card';
import { Chip } from '@/shared/ui/Chip';
import { Avatar, Divider, Field, ListRow, SectionTitle } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { ErrorState, SkeletonList } from '@/shared/ui/States';

/** Lebane ID profile and memberships (R-ID-1, R-ORG-1). */
export function Profile() {
  const { t } = useTranslation();
  const me = useMe();
  const orgs = useOrgs();
  return (
    <Screen title={t('profile.title')} back>
      {me.isLoading ? <SkeletonList rows={2} /> : null}
      {me.isError ? <ErrorState error={me.error} onRetry={() => void me.refetch()} /> : null}
      {me.data ? (
        <Card className="gap-2">
          <View className="flex-row items-center gap-3">
            <Avatar name={me.data.fullName} />
            <AppText variant="subheading">{me.data.fullName}</AppText>
          </View>
          <Field label={t('fields.email')} value={me.data.email} />
        </Card>
      ) : null}
      <SectionTitle title={t('profile.memberships')} />
      <Card className="py-1">
        {(orgs.data ?? []).map((org, index) => (
          <View key={org.id}>
            {index > 0 ? <Divider /> : null}
            <ListRow title={org.legalName} leading={<Avatar name={org.legalName} />} trailing={<Chip label={t(`roles.${org.role}`)} tone="brand" />} />
          </View>
        ))}
      </Card>
    </Screen>
  );
}
