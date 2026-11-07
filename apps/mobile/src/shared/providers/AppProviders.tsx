import { JetBrainsMono_400Regular, JetBrainsMono_500Medium } from '@expo-google-fonts/jetbrains-mono';
import {
  PlusJakartaSans_400Regular,
  PlusJakartaSans_500Medium,
  PlusJakartaSans_600SemiBold,
  PlusJakartaSans_700Bold,
  useFonts,
} from '@expo-google-fonts/plus-jakarta-sans';
import { PersistQueryClientProvider } from '@tanstack/react-query-persist-client';
import * as SplashScreen from 'expo-splash-screen';
import { StatusBar } from 'expo-status-bar';
import { type ReactNode, useEffect } from 'react';
import { SafeAreaProvider } from 'react-native-safe-area-context';

import '@/shared/i18n';
import { wireQueryLifecycle } from '@/shared/lifecycle/queryLifecycle';
import { PERSIST_BUSTER, queryClient, queryPersister } from '@/shared/queries/queryClient';
import { LivePayinBanner } from '@/shared/realtime/LivePayinBanner';
import { useBootSession } from '@/shared/session/useBootSession';
import { StepUpSheet } from '@/shared/stepup/StepUpSheet';
import { useSessionStore } from '@/shared/stores/useSessionStore';
import { ToastHost } from '@/shared/ui/ToastHost';

void SplashScreen.preventAutoHideAsync().catch(() => undefined);
wireQueryLifecycle();

const PERSIST_MAX_AGE_MS = 24 * 60 * 60 * 1000;

/**
 * Root providers: fonts, safe areas, TanStack Query with persistence, the session boot and
 * the global hosts (step-up sheet, toasts, live payin banner).
 */
export function AppProviders({ children }: { children: ReactNode }) {
  const [fontsLoaded, fontError] = useFonts({
    PlusJakartaSans_400Regular,
    PlusJakartaSans_500Medium,
    PlusJakartaSans_600SemiBold,
    PlusJakartaSans_700Bold,
    JetBrainsMono_400Regular,
    JetBrainsMono_500Medium,
  });
  useBootSession();
  const booting = useSessionStore((state) => state.status === 'booting');
  const ready = (fontsLoaded || Boolean(fontError)) && !booting;

  useEffect(() => {
    if (ready) void SplashScreen.hideAsync().catch(() => undefined);
  }, [ready]);

  return (
    <SafeAreaProvider>
      <PersistQueryClientProvider client={queryClient} persistOptions={{ persister: queryPersister, buster: PERSIST_BUSTER, maxAge: PERSIST_MAX_AGE_MS }}>
        <StatusBar style="auto" />
        {ready ? children : null}
        <StepUpSheet />
        <LivePayinBanner />
        <ToastHost />
      </PersistQueryClientProvider>
    </SafeAreaProvider>
  );
}
