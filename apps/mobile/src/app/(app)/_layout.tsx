import { Stack } from 'expo-router';

import { LiveEventsBridge } from '@/shared/realtime/LiveEventsBridge';

export default function SignedInLayout() {
  return (
    <>
      <LiveEventsBridge />
      <Stack screenOptions={{ headerShown: false, animation: 'slide_from_right' }}>
        <Stack.Screen name="(tabs)" />
        <Stack.Screen name="orgs" options={{ presentation: 'modal', animation: 'slide_from_bottom' }} />
        <Stack.Screen name="pay" options={{ presentation: 'fullScreenModal', animation: 'slide_from_bottom', gestureEnabled: false }} />
        <Stack.Screen name="move" options={{ presentation: 'modal', animation: 'slide_from_bottom' }} />
      </Stack>
    </>
  );
}
