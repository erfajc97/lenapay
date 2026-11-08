import { Tabs } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { selectionHaptic } from '@/shared/feedback/haptics';
import { useThemeColor } from '@/shared/ui/Icon';
import { TabBarBackground, TabIcon } from '@/shared/ui/TabIcon';

const BAR_HEIGHT = 80;

export default function TabsLayout() {
  const { t } = useTranslation();
  const active = useThemeColor('primary');
  const inactive = useThemeColor('text-muted');
  const insets = useSafeAreaInsets();
  return (
    <Tabs
      // A light tick only when the tab actually changes, not on a re-tap of the current one.
      screenListeners={({ navigation }) => ({
        tabPress: () => {
          if (!navigation.isFocused()) selectionHaptic();
        },
      })}
      screenOptions={{
        headerShown: false,
        tabBarActiveTintColor: active,
        tabBarInactiveTintColor: inactive,
        // The bar's surface, rounded corners and shadow come from TabBarBackground (one elevation helper).
        // Taller than the default so the popped (risen) icon and its label both fit above the home indicator.
        tabBarStyle: { backgroundColor: 'transparent', borderTopWidth: 0, elevation: 0, height: BAR_HEIGHT + insets.bottom, paddingBottom: Math.max(insets.bottom, 12) },
        tabBarBackground: () => <TabBarBackground />,
        tabBarLabelStyle: { fontFamily: 'PlusJakartaSans_600SemiBold', fontSize: 10, marginTop: 2 },
        // Room for the 44 dp circle of the selected tab, which rises above the bar's top edge.
        tabBarIconStyle: { width: 44, height: 36, overflow: 'visible' },
        tabBarItemStyle: { paddingTop: 10, paddingHorizontal: 0, overflow: 'visible' },
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
