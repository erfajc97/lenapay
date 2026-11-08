import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { Card } from '@/shared/ui/Card';
import { Divider, IconBadge, ListRow, SectionTitle } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';

import { type MoreItem, useMore } from './hooks/useMore';

/** "Más" tab: Mis pagos and the organization's products and management (preferences live in Configuración). */
export function More() {
  const { t } = useTranslation();
  const more = useMore();
  const subtitle = (item: MoreItem) => (item.subtitle === null ? null : 'text' in item.subtitle ? item.subtitle.text : t(item.subtitle.key, item.subtitle.params));

  return (
    <Screen title={t('settings.title')} inTabs>
      {more.hasMyPayments ? (
        <>
          <SectionTitle title={t('settings.personal')} />
          <Card className="py-1">
            <ListRow title={t('myPayments.title')} subtitle={t('home.myPayments.body')} leading={<IconBadge name="business-outline" />} onPress={() => more.open('/my-payments')} />
          </Card>
        </>
      ) : null}

      <SectionTitle title={more.org?.legalName ?? t('settings.organization')} />
      <Card className="py-1">
        {more.orgItems.map((item, index) => (
          <View key={item.key}>
            {index > 0 ? <Divider /> : null}
            <ListRow title={t(item.title)} subtitle={subtitle(item)} leading={<IconBadge name={item.icon} />} onPress={() => more.open(item.href)} />
          </View>
        ))}
      </Card>
    </Screen>
  );
}
