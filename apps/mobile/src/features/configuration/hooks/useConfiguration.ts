import { router } from 'expo-router';

import { config } from '@/shared/config/env';
import { displayName } from '@/shared/helpers/people';
import { serverLanguage } from '@/shared/helpers/preferences';
import type { LanguagePreference } from '@/shared/i18n/locale';
import { useLogout } from '@/shared/mutations/useLogout';
import { useAccounts } from '@/shared/queries/accounts';
import { useOrgs } from '@/shared/queries/orgs';
import { useMe } from '@/shared/queries/session';
import { usePreferencesStore } from '@/shared/stores/usePreferencesStore';
import type { ThemePreference } from '@/shared/theme/themePreference';

import { useSyncAppearance } from '../mutations/useSyncAppearance';

/**
 * Configuración (owner feedback): the person's own preferences, apart from "Más". A language or
 * theme picked here applies at once and is synced to the server (R-ID-5).
 */
export function useConfiguration() {
  const me = useMe();
  const orgs = useOrgs();
  const preferences = me.data?.preferences;
  const defaultOrg = orgs.data?.find((org) => org.id === preferences?.defaultOrgId) ?? null;
  const accounts = useAccounts(defaultOrg?.id);
  const defaultAccount = accounts.data?.find((account) => account.id === preferences?.defaultAccountId) ?? null;
  const theme = usePreferencesStore((state) => state.themePreference);
  const setTheme = usePreferencesStore((state) => state.setThemePreference);
  const language = usePreferencesStore((state) => state.languagePreference);
  const setLanguage = usePreferencesStore((state) => state.setLanguagePreference);
  const preview = usePreferencesStore((state) => state.personalModePreview);
  const setPreview = usePreferencesStore((state) => state.setPersonalModePreview);
  const sync = useSyncAppearance();
  const logout = useLogout();

  return {
    me,
    name: me.data ? displayName(me.data) : null,
    defaultOrgName: defaultOrg?.legalName ?? null,
    defaultAccountName: defaultAccount?.name ?? null,
    theme,
    setTheme: (value: ThemePreference) => {
      setTheme(value);
      sync.mutate({ theme: value });
    },
    language,
    setLanguage: (value: LanguagePreference) => {
      setLanguage(value);
      sync.mutate({ locale: serverLanguage(value) });
    },
    preview,
    setPreview,
    devTools: config.devTools,
    version: config.appVersion,
    open: (href: '/settings/profile' | '/settings/default-account' | '/settings/security' | '/developer') => router.push(href),
    logout: () => logout.mutate(),
    loggingOut: logout.isPending,
  };
}
