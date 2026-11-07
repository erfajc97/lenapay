import { Tabs } from 'expo-router';
import { useTranslation } from 'react-i18next';

import { useThemeColor } from '@/shared/ui/Icon';
import { TabBarBackground, TabIcon } from '@/shared/ui/TabIcon';

export default function TabsLayout() {
  const { t } = useTranslation();
  const active = useThemeColor('primary');
  const inactive = useThemeColor('text-muted');
  return (
    <Tabs
      screenOptions={{
        headerShown: false,
        tabBarActiveTintColor: active,
        tabBarInactiveTintColor: inactive,
        // The bar's surface, rounded corners and shadow come from TabBarBackground (one elevation helper).
        tabBarStyle: { backgroundColor: 'transparent', borderTopWidth: 0, elevation: 0 },
        tabBarBackground: () => <TabBarBackground />,
        tabBarLabelStyle: { fontFamily: 'PlusJakartaSans_500Medium', fontSize: 11 },
      }}
    >
      <Tabs.Screen name="index" options={{ title: t('tabs.home'), tabBarIcon: ({ color, focused }) => <TabIcon name="home" color={color} focused={focused} /> }} />
      <Tabs.Screen name="accounts" options={{ title: t('tabs.accounts'), tabBarIcon: ({ color, focused }) => <TabIcon name="wallet" color={color} focused={focused} /> }} />
      <Tabs.Screen name="payers" options={{ title: t('tabs.payers'), tabBarIcon: ({ color, focused }) => <TabIcon name="people" color={color} focused={focused} /> }} />
      <Tabs.Screen name="movements" options={{ title: t('tabs.movements'), tabBarIcon: ({ color, focused }) => <TabIcon name="swap-vertical" color={color} focused={focused} /> }} />
      <Tabs.Screen name="settings" options={{ title: t('tabs.more'), tabBarIcon: ({ color, focused }) => <TabIcon name="grid" color={color} focused={focused} /> }} />
    </Tabs>
  );
}
