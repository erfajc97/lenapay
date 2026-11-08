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
import { type ReactNode, useCallback, useState } from 'react';
import { SafeAreaProvider } from 'react-native-safe-area-context';

import '@/shared/i18n';
import { useApplyLanguagePreference } from '@/shared/i18n/useApplyLanguagePreference';
import { wireQueryLifecycle } from '@/shared/lifecycle/queryLifecycle';
import { PERSIST_BUSTER, queryClient, queryPersister } from '@/shared/queries/queryClient';
import { LivePayinBanner } from '@/shared/realtime/LivePayinBanner';
import { useBootSession } from '@/shared/session/useBootSession';
import { StepUpSheet } from '@/shared/stepup/StepUpSheet';
import { usePreferencesHydrated } from '@/shared/stores/usePreferencesStore';
import { useSessionStore } from '@/shared/stores/useSessionStore';
import { useApplyThemePreference } from '@/shared/theme/useApplyThemePreference';
import { LaunchScreen } from '@/shared/ui/LaunchScreen';
import { ToastHost } from '@/shared/ui/ToastHost';

void SplashScreen.preventAutoHideAsync().catch(() => undefined);
wireQueryLifecycle();

const PERSIST_MAX_AGE_MS = 24 * 60 * 60 * 1000;

/**
 * Root providers: fonts, safe areas, TanStack Query with persistence, the session boot, the
 * theme and language picked in Settings and the global hosts (step-up sheet, toasts, live payin banner).
 *
 * Start-up: the native splash stays until the fonts load and the JS launch screen is on screen;
 * the launch screen then covers preferences hydration and session restore, and crossfades into
 * the app (already rendered underneath) once everything is ready.
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
  const statusBarStyle = useApplyThemePreference();
  useApplyLanguagePreference();
  const booting = useSessionStore((state) => state.status === 'booting');
  const preferencesHydrated = usePreferencesHydrated();
  const fontsReady = fontsLoaded || Boolean(fontError);
  const ready = fontsReady && preferencesHydrated && !booting;
  const [launching, setLaunching] = useState(true);
  const hideNativeSplash = useCallback(() => {
    SplashScreen.hideAsync().catch(() => undefined);
  }, []);
  const launchDone = useCallback(() => setLaunching(false), []);

  return (
    <SafeAreaProvider>
      <PersistQueryClientProvider client={queryClient} persistOptions={{ persister: queryPersister, buster: PERSIST_BUSTER, maxAge: PERSIST_MAX_AGE_MS }}>
        <StatusBar style={statusBarStyle} />
        {ready ? children : null}
        {fontsReady && launching ? <LaunchScreen ready={ready} onShown={hideNativeSplash} onDone={launchDone} /> : null}
        <StepUpSheet />
        <LivePayinBanner />
        <ToastHost />
      </PersistQueryClientProvider>
    </SafeAreaProvider>
  );
}
