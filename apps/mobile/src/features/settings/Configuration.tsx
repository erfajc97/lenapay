import { useTranslation } from 'react-i18next';
import { Switch, View } from 'react-native';

import { LANGUAGE_LABEL_KEY, LANGUAGE_PREFERENCES } from '@/shared/i18n/locale';
import { THEME_PREFERENCES } from '@/shared/theme/themePreference';
import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { DemoBadge } from '@/shared/ui/DemoBadge';
import { Icon } from '@/shared/ui/Icon';
import { Avatar, Divider, IconBadge, ListRow, SectionTitle } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { Segmented } from '@/shared/ui/Segmented';

import { useConfiguration } from './hooks/useConfiguration';

/** Configuración: profile, default account, appearance, language, security, notifications, developer, about, logout. */
export function Configuration() {
  const { t } = useTranslation();
  const screen = useConfiguration();
  const defaultSubtitle =
    screen.defaultOrgName === null
      ? t('settings.defaultAccountNone')
      : screen.defaultAccountName === null
        ? screen.defaultOrgName
        : t('defaultAccount.current', { org: screen.defaultOrgName, account: screen.defaultAccountName });

  return (
    <Screen title={t('config.title')} back>
      <SectionTitle title={t('config.account')} />
      <Card className="py-1">
        <ListRow
          title={screen.name ?? t('common.loading')}
          subtitle={screen.me.data?.fullName ? screen.me.data.email : null}
          leading={<Avatar name={screen.name ?? ''} />}
          onPress={() => screen.open('/settings/profile')}
        />
        <Divider />
        <ListRow title={t('defaultAccount.title')} subtitle={defaultSubtitle} leading={<IconBadge name="star-outline" />} onPress={() => screen.open('/settings/default-account')} />
      </Card>

      <SectionTitle title={t('settings.appearance')} />
      <Card className="gap-3">
        <AppText tone="secondary">{t('settings.appearanceBody')}</AppText>
        <Segmented
          label={t('settings.appearance')}
          value={screen.theme}
          onChange={screen.setTheme}
          options={THEME_PREFERENCES.map((value) => ({ value, label: t(`settings.theme.${value}`) }))}
        />
      </Card>

      <SectionTitle title={t('settings.language')} />
      <Card className="gap-1 py-2">
        <AppText tone="secondary" className="pb-1">
          {t('settings.languageBody')}
        </AppText>
        <View accessibilityRole="radiogroup" accessibilityLabel={t('settings.language')}>
          {LANGUAGE_PREFERENCES.map((value, index) => (
            <View key={value}>
              {index > 0 ? <Divider /> : null}
              <ListRow
                title={t(LANGUAGE_LABEL_KEY[value])}
                onPress={() => screen.setLanguage(value)}
                chevron={false}
                selected={screen.language === value}
                trailing={screen.language === value ? <Icon name="checkmark" size={20} tone="primary" /> : null}
              />
            </View>
          ))}
        </View>
      </Card>

      <SectionTitle title={t('config.preferences')} />
      <Card className="py-1">
        <ListRow title={t('settings.security')} subtitle={t('config.securityBody')} leading={<IconBadge name="lock-closed-outline" />} onPress={() => screen.open('/settings/security')} />
        <Divider />
        <ListRow title={t('config.notifications')} subtitle={t('config.notificationsBody')} leading={<IconBadge name="notifications-outline" />} onPress={() => screen.open('/settings/security')} />
        <Divider />
        <ListRow
          title={t('settings.personalPreview')}
          subtitle={t('settings.personalPreviewBody')}
          leading={<IconBadge name="person-outline" />}
          trailing={<Switch value={screen.preview} onValueChange={screen.setPreview} accessibilityLabel={t('settings.personalPreview')} trackColorOnClassName="accent-primary" />}
        />
        {screen.devTools ? (
          <>
            <Divider />
            <ListRow title={t('developer.title')} subtitle={t('developer.rowBody')} leading={<IconBadge name="construct-outline" />} onPress={() => screen.open('/developer')} />
          </>
        ) : null}
      </Card>

      <SectionTitle title={t('config.about')} />
      <View className="items-center gap-2">
        <DemoBadge />
        <AppText variant="caption" tone="muted">
          {t('config.aboutBody', { version: screen.version })}
        </AppText>
      </View>
      <Button label={t('settings.logout')} variant="danger" icon="log-out-outline" onPress={screen.logout} loading={screen.loggingOut} />
    </Screen>
  );
}
