import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { Chip } from '@/shared/ui/Chip';
import { Icon } from '@/shared/ui/Icon';
import { Avatar, Divider, IconBadge, ListRow, SectionTitle } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { EmptyState, ErrorState, SkeletonList } from '@/shared/ui/States';

import { useDefaultAccount } from './hooks/useDefaultAccount';

const check = <Icon name="checkmark" size={20} tone="primary" />;

/** Configuración → Cuenta predeterminada (R-ID-5): where the app opens, and the account Pagar/Mover start with. */
export function DefaultAccount() {
  const { t } = useTranslation();
  const screen = useDefaultAccount();
  return (
    <Screen
      title={t('defaultAccount.title')}
      back
      footer={<Button label={t('common.save')} onPress={screen.save} loading={screen.saving} disabled={!screen.dirty || screen.saving} />}
    >
      <AppText tone="secondary">{t('defaultAccount.intro')}</AppText>
      {screen.me.isLoading || screen.orgsQuery.isLoading ? <SkeletonList rows={3} /> : null}
      {screen.orgsQuery.isError ? <ErrorState error={screen.orgsQuery.error} onRetry={() => void screen.orgsQuery.refetch()} /> : null}

      <SectionTitle title={t('defaultAccount.orgLabel')} />
      <Card className="py-1">
        <View accessibilityRole="radiogroup" accessibilityLabel={t('defaultAccount.orgLabel')}>
          <ListRow
            title={t('defaultAccount.none')}
            subtitle={t('defaultAccount.noneBody')}
            leading={<IconBadge name="time-outline" />}
            onPress={() => screen.pickOrg(null)}
            chevron={false}
            selected={screen.orgId === null}
            trailing={screen.orgId === null ? check : null}
          />
          {screen.orgs.map((org) => (
            <View key={org.id}>
              <Divider />
              <ListRow
                title={org.legalName}
                subtitle={t(`roles.${org.role}`)}
                leading={<Avatar name={org.legalName} />}
                onPress={() => screen.pickOrg(org.id)}
                chevron={false}
                selected={screen.orgId === org.id}
                trailing={screen.orgId === org.id ? check : null}
              />
            </View>
          ))}
        </View>
      </Card>

      {screen.orgId !== null ? (
        <>
          <SectionTitle title={t('defaultAccount.accountLabel')} />
          {screen.accounts.isLoading ? <SkeletonList rows={2} /> : null}
          {screen.accounts.isError ? <ErrorState error={screen.accounts.error} onRetry={() => void screen.accounts.refetch()} /> : null}
          {screen.accounts.isSuccess && screen.openAccounts.length === 0 ? <EmptyState icon="wallet-outline" title={t('defaultAccount.noAccounts')} /> : null}
          {screen.openAccounts.length > 0 ? (
            <Card className="py-1">
              <View accessibilityRole="radiogroup" accessibilityLabel={t('defaultAccount.accountLabel')}>
                {screen.openAccounts.map((account, index) => (
                  <View key={account.id}>
                    {index > 0 ? <Divider /> : null}
                    <ListRow
                      title={account.name}
                      leading={<IconBadge name="wallet-outline" />}
                      onPress={() => screen.pickAccount(account.id)}
                      chevron={false}
                      selected={screen.accountId === account.id}
                      trailing={
                        <View className="flex-row items-center gap-2">
                          <Chip label={t(`accounts.kind.${account.kind}`)} tone={account.kind === 'project' ? 'brand' : 'neutral'} />
                          {screen.accountId === account.id ? check : null}
                        </View>
                      }
                    />
                  </View>
                ))}
              </View>
            </Card>
          ) : null}
        </>
      ) : null}
    </Screen>
  );
}
