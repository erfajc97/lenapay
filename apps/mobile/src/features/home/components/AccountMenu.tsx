import { useTranslation } from 'react-i18next';
import { Pressable, View } from 'react-native';

import { AppText } from '@/shared/ui/AppText';
import { Icon, type IconName } from '@/shared/ui/Icon';
import { Popover } from '@/shared/ui/Popover';
import { Avatar, Divider } from '@/shared/ui/Rows';

export type AccountMenuTarget = 'profile' | 'switchOrg' | 'defaultAccount' | 'settings' | 'security';

const ITEMS: { target: AccountMenuTarget; icon: IconName; label: string }[] = [
  { target: 'profile', icon: 'person-circle-outline', label: 'menu.profile' },
  { target: 'switchOrg', icon: 'swap-horizontal', label: 'menu.switchOrg' },
  { target: 'defaultAccount', icon: 'star-outline', label: 'defaultAccount.title' },
  { target: 'settings', icon: 'settings-outline', label: 'menu.settings' },
  { target: 'security', icon: 'lock-closed-outline', label: 'menu.security' },
];

function MenuItem({ icon, label, danger, onPress }: { icon: IconName; label: string; danger?: boolean; onPress: () => void }) {
  return (
    <Pressable onPress={onPress} accessibilityRole="menuitem" accessibilityLabel={label} className="min-h-12 flex-row items-center gap-3 px-4 active:bg-primary-soft">
      <Icon name={icon} size={20} tone={danger ? 'danger' : 'text-secondary'} />
      <AppText tone={danger ? 'danger' : 'primary'} className="flex-1">
        {label}
      </AppText>
    </Pressable>
  );
}

interface Props {
  visible: boolean;
  name: string;
  email: string;
  orgName: string;
  confirmLogout: boolean;
  onPick: (target: AccountMenuTarget) => void;
  onLogout: () => void;
  onClose: () => void;
  onDismissed: () => void;
}

/** The avatar's menu (owner feedback): who I am and where I am, then my account's places and logout (tap twice). */
export function AccountMenu({ visible, name, email, orgName, confirmLogout, onPick, onLogout, onClose, onDismissed }: Props) {
  const { t } = useTranslation();
  return (
    <Popover visible={visible} label={t('menu.label')} onClose={onClose} onDismissed={onDismissed}>
      <View className="flex-row items-center gap-3 px-4 py-3" accessible accessibilityLabel={`${name}, ${email}, ${orgName}`}>
        <Avatar name={name} />
        <View className="flex-1 gap-0.5">
          <AppText variant="bodyStrong" numberOfLines={1}>
            {name}
          </AppText>
          <AppText variant="caption" tone="muted" numberOfLines={1}>
            {email}
          </AppText>
          <AppText variant="caption" tone="brand" numberOfLines={1}>
            {orgName}
          </AppText>
        </View>
      </View>
      <Divider />
      {ITEMS.map((item) => (
        <MenuItem key={item.target} icon={item.icon} label={t(item.label)} onPress={() => onPick(item.target)} />
      ))}
      <Divider />
      <MenuItem icon="log-out-outline" label={t(confirmLogout ? 'menu.confirmLogout' : 'menu.logout')} danger onPress={onLogout} />
    </Popover>
  );
}
