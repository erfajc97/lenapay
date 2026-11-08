import { Tabs } from 'expo-router';
import { useTranslation } from 'react-i18next';

import { selectionHaptic } from '@/shared/feedback/haptics';
import { FloatingTabBar, TabIcon } from '@/shared/ui/TabIcon';

export default function TabsLayout() {
  const { t } = useTranslation();
  return (
    <Tabs
      // A light tick only when the tab actually changes, not on a re-tap of the current one.
      screenListeners={({ navigation }) => ({
        tabPress: () => {
          if (!navigation.isFocused()) selectionHaptic();
        },
      })}
      // Floating capsule with the liquid indicator; screens reserve its space (tabBarSpace).
      tabBar={(props) => <FloatingTabBar {...props} />}
      screenOptions={{ headerShown: false }}
    >
      <Tabs.Screen name="index" options={{ title: t('tabs.home'), tabBarIcon: ({ focused }) => <TabIcon name="home" focused={focused} /> }} />
      <Tabs.Screen name="accounts" options={{ title: t('tabs.accounts'), tabBarIcon: ({ focused }) => <TabIcon name="wallet" focused={focused} /> }} />
      <Tabs.Screen name="payers" options={{ title: t('tabs.payers'), tabBarIcon: ({ focused }) => <TabIcon name="people" focused={focused} /> }} />
      <Tabs.Screen name="movements" options={{ title: t('tabs.movements'), tabBarIcon: ({ focused }) => <TabIcon name="swap-vertical" focused={focused} /> }} />
      <Tabs.Screen name="settings" options={{ title: t('tabs.more'), tabBarIcon: ({ focused }) => <TabIcon name="grid" focused={focused} /> }} />
    </Tabs>
  );
}
