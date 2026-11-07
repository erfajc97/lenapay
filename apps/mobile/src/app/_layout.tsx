import '../../global.css';

import { Stack, type ErrorBoundaryProps } from 'expo-router';

import { AppProviders } from '@/shared/providers/AppProviders';
import { useSessionStore } from '@/shared/stores/useSessionStore';
import { RootError } from '@/shared/ui/RootError';

export function ErrorBoundary({ error, retry }: ErrorBoundaryProps) {
  return <RootError error={error} retry={retry} />;
}

function RootStack() {
  const status = useSessionStore((state) => state.status);
  return (
    <Stack screenOptions={{ headerShown: false, animation: 'fade' }}>
      <Stack.Protected guard={status === 'signedOut'}>
        <Stack.Screen name="(auth)" />
      </Stack.Protected>
      <Stack.Protected guard={status === 'locked'}>
        <Stack.Screen name="lock" />
      </Stack.Protected>
      <Stack.Protected guard={status === 'active'}>
        <Stack.Screen name="(app)" />
      </Stack.Protected>
    </Stack>
  );
}

export default function RootLayout() {
  return (
    <AppProviders>
      <RootStack />
    </AppProviders>
  );
}
