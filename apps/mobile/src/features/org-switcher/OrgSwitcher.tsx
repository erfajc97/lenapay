import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { formatCuit } from '@/shared/helpers/cuit';
import { KYC_STATUS_TONE } from '@/shared/helpers/status';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { Chip } from '@/shared/ui/Chip';
import { Icon } from '@/shared/ui/Icon';
import { Avatar, Divider, ListRow } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { ErrorState, SkeletonList } from '@/shared/ui/States';

import { useOrgSwitcher } from './hooks/useOrgSwitcher';

/** Organizations of the user with role and KYC status (R-ORG-1). */
export function OrgSwitcher() {
  const { t } = useTranslation();
  const screen = useOrgSwitcher();
  return (
    <Screen title={t('orgs.title')} back footer={<Button label={t('orgs.create')} variant="secondary" icon="add" onPress={screen.create} />}>
      {screen.query.isLoading ? <SkeletonList rows={3} /> : null}
      {screen.query.isError ? <ErrorState error={screen.query.error} onRetry={() => void screen.query.refetch()} /> : null}
      <Card className="py-1">
        {screen.orgs.map((org, index) => (
          <View key={org.id}>
            {index > 0 ? <Divider /> : null}
            <ListRow
              title={org.legalName}
              subtitle={`${t(`roles.${org.role}`)}${org.cuit ? ` · ${formatCuit(org.cuit)}` : ''}`}
              leading={<Avatar name={org.legalName} />}
              trailing={
                <View className="items-end gap-1">
                  {org.id === screen.activeId ? <Icon name="checkmark-circle" size={22} tone="primary" /> : null}
                  {org.kycStatus !== 'approved' ? <Chip label={t(`status.kyc.${org.kycStatus}`)} tone={KYC_STATUS_TONE[org.kycStatus]} /> : null}
                  {org.kind === 'personal' ? <Chip label={t('orgs.personal')} tone="brand" /> : null}
                </View>
              }
              onPress={() => screen.select(org)}
              chevron={false}
              accessibilityLabel={t('orgs.selectA11y', { name: org.legalName })}
            />
          </View>
        ))}
      </Card>
    </Screen>
  );
}
