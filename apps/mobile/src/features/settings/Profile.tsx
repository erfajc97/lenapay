import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { displayName } from '@/shared/helpers/people';
import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { Chip } from '@/shared/ui/Chip';
import { Avatar, Divider, Field, ListRow, SectionTitle } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { ErrorState, SkeletonList } from '@/shared/ui/States';
import { TextField } from '@/shared/ui/TextField';

import { useProfile } from './hooks/useProfile';

/** Lebane ID profile: full name (PATCH /me, R-ID-5), email and memberships (R-ID-1, R-ORG-1). */
export function Profile() {
  const { t } = useTranslation();
  const profile = useProfile();
  const me = profile.me;
  return (
    <Screen title={t('profile.title')} back>
      {me.isLoading ? <SkeletonList rows={2} /> : null}
      {me.isError ? <ErrorState error={me.error} onRetry={() => void me.refetch()} /> : null}
      {me.data ? (
        <Card className="gap-3">
          <View className="flex-row items-center gap-3">
            <Avatar name={displayName(me.data)} />
            <AppText variant="subheading" className="flex-1" numberOfLines={2}>
              {displayName(me.data)}
            </AppText>
          </View>
          <TextField
            label={t('fields.fullName')}
            value={profile.name}
            onChangeText={profile.setName}
            error={profile.nameError ? t(profile.nameError) : null}
            hint={t('profile.nameHint')}
            autoCapitalize="words"
            autoComplete="name"
            textContentType="name"
            returnKeyType="done"
            onSubmitEditing={profile.canSave ? profile.save : undefined}
            maxLength={140}
          />
          <Button label={t('profile.saveName')} onPress={profile.save} loading={profile.saving} disabled={!profile.canSave} compact />
          <Field label={t('fields.email')} value={me.data.email} />
        </Card>
      ) : null}
      <SectionTitle title={t('profile.memberships')} />
      <Card className="py-1">
        {(profile.orgs.data ?? []).map((org, index) => (
          <View key={org.id}>
            {index > 0 ? <Divider /> : null}
            <ListRow
              title={org.legalName}
              leading={<Avatar name={org.legalName} />}
              trailing={<Chip label={t(`roles.${org.role}`)} tone={org.role === 'owner' ? 'success' : 'brand'} />}
            />
          </View>
        ))}
      </Card>
    </Screen>
  );
}
