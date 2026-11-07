import { useTranslation } from 'react-i18next';
import { Pressable, View } from 'react-native';

import type { Org } from '@/shared/api/types';
import { AppText } from '@/shared/ui/AppText';
import { Icon } from '@/shared/ui/Icon';
import { Avatar } from '@/shared/ui/Rows';
import { Wordmark } from '@/shared/ui/Wordmark';

/** Brand row (Lebane isotype + "Lenapay") with the profile avatar, then greeting + org name; tapping the org opens the switcher. */
export function OrgHeader({ org, firstName, onSwitch, onProfile }: { org: Org; firstName: string | null; onSwitch: () => void; onProfile: () => void }) {
  const { t } = useTranslation();
  return (
    <View className="gap-3 pt-2">
      <View className="flex-row items-center justify-between">
        <Wordmark size="small" />
        <Pressable onPress={onProfile} accessibilityRole="button" accessibilityLabel={t('home.profile')} hitSlop={6}>
          <Avatar name={firstName ?? org.legalName} />
        </Pressable>
      </View>
      <Pressable onPress={onSwitch} accessibilityRole="button" accessibilityLabel={t('home.switchOrg', { name: org.legalName })} className="gap-0.5">
        <AppText variant="label" tone="muted">
          {firstName ? t('home.greeting', { name: firstName }) : t('home.greetingAnonymous')}
        </AppText>
        <View className="flex-row items-center gap-1">
          <AppText variant="heading" numberOfLines={1} className="shrink">
            {org.legalName}
          </AppText>
          <Icon name="chevron-down" size={18} tone="text-secondary" />
        </View>
      </Pressable>
    </View>
  );
}
