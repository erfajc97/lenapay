import { router } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { Switch, View } from 'react-native';

import { config } from '@/shared/config/env';
import { canManageTeam, canSeeReconciliation } from '@/shared/helpers/permissions';
import { useErpLink } from '@/shared/queries/erp';
import { useActiveOrg } from '@/shared/queries/orgs';
import { useMe } from '@/shared/queries/session';
import { usePreferencesStore } from '@/shared/stores/usePreferencesStore';
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

import { useLogout } from './mutations/useSettingsMutations';

/** "Más": profile, organization tools, security, appearance, language, personal-mode preview, logout. */
export function Settings() {
  const { t } = useTranslation();
  const me = useMe();
  const { org } = useActiveOrg();
  const preview = usePreferencesStore((state) => state.personalModePreview);
  const setPreview = usePreferencesStore((state) => state.setPersonalModePreview);
  const theme = usePreferencesStore((state) => state.themePreference);
  const setTheme = usePreferencesStore((state) => state.setThemePreference);
  const language = usePreferencesStore((state) => state.languagePreference);
  const setLanguage = usePreferencesStore((state) => state.setLanguagePreference);
  const logout = useLogout();
  const erpLink = useErpLink(org?.kind === 'company' && org.kycStatus === 'approved' ? org.id : undefined);
  const role = org?.role ?? 'viewer';

  const orgItems = [
    { key: 'switch', icon: 'swap-horizontal' as const, title: t('settings.switchOrg'), subtitle: org?.legalName, href: '/orgs' as const, show: true },
    { key: 'approvals', icon: 'checkmark-done-outline' as const, title: t('settings.approvals'), subtitle: null, href: '/approvals' as const, show: true },
    { key: 'team', icon: 'people-outline' as const, title: t('settings.team'), subtitle: canManageTeam(role) ? null : t('settings.readOnly'), href: '/team' as const, show: true },
    { key: 'recon', icon: 'git-compare-outline' as const, title: t('settings.reconciliation'), subtitle: null, href: '/reconciliation' as const, show: canSeeReconciliation(role) },
    { key: 'erp', icon: 'git-merge-outline' as const, title: t('settings.erp'), subtitle: erpLink.data?.status === 'linked' ? t('settings.erpLinked') : null, href: '/erp-link' as const, show: true },
    { key: 'kyc', icon: 'shield-checkmark-outline' as const, title: t('settings.kyc'), subtitle: org ? t(`status.kyc.${org.kycStatus}`) : null, href: '/onboarding/status' as const, show: true },
  ].filter((item) => item.show);

  return (
    <Screen title={t('settings.title')} inTabs>
      <Card className="py-1">
        <ListRow
          title={me.data ? (me.data.fullName ?? me.data.email) : t('common.loading')}
          subtitle={me.data?.email}
          leading={<Avatar name={me.data?.fullName ?? me.data?.email ?? ''} />}
          onPress={() => router.push('/settings/profile')}
        />
      </Card>

      <SectionTitle title={org?.legalName ?? t('settings.organization')} />
      <Card className="py-1">
        {orgItems.map((item, index) => (
          <View key={item.key}>
            {index > 0 ? <Divider /> : null}
            <ListRow title={item.title} subtitle={item.subtitle} leading={<IconBadge name={item.icon} />} onPress={() => router.push(item.href)} />
          </View>
        ))}
      </Card>

      <SectionTitle title={t('settings.app')} />
      <Card className="py-1">
        <ListRow title={t('settings.security')} leading={<IconBadge name="lock-closed-outline" />} onPress={() => router.push('/settings/security')} />
        <Divider />
        {config.devTools ? (
          <>
            <ListRow title={t('developer.title')} subtitle={t('developer.rowBody')} leading={<IconBadge name="construct-outline" />} onPress={() => router.push('/developer')} />
            <Divider />
          </>
        ) : null}
        <ListRow
          title={t('settings.personalPreview')}
          subtitle={t('settings.personalPreviewBody')}
          leading={<IconBadge name="person-outline" />}
          trailing={<Switch value={preview} onValueChange={setPreview} accessibilityLabel={t('settings.personalPreview')} trackColorOnClassName="accent-primary" />}
        />
      </Card>

      <SectionTitle title={t('settings.appearance')} />
      <Card className="gap-3">
        <AppText tone="secondary">{t('settings.appearanceBody')}</AppText>
        <Segmented
          label={t('settings.appearance')}
          value={theme}
          onChange={setTheme}
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
                onPress={() => setLanguage(value)}
                chevron={false}
                selected={language === value}
                trailing={language === value ? <Icon name="checkmark" size={20} tone="primary" /> : null}
              />
            </View>
          ))}
        </View>
      </Card>

      <View className="items-center gap-2 pt-2">
        <DemoBadge />
        <AppText variant="caption" tone="muted">
          {t('settings.version', { version: config.appVersion })}
        </AppText>
      </View>
      <Button label={t('settings.logout')} variant="danger" icon="log-out-outline" onPress={() => logout.mutate()} loading={logout.isPending} />
    </Screen>
  );
}
