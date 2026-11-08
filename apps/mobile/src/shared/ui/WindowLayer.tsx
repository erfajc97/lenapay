import { Fragment, type ReactNode, useEffect, useRef, useState } from 'react';
import { Platform, StyleSheet, View } from 'react-native';
import Animated, { FadeIn, FadeOut } from 'react-native-reanimated';
import { FullWindowOverlay } from 'react-native-screens';

import { rootSheetUsesWindowOverlay } from '@/shared/helpers/platform';

/**
 * Layers mounted at the root (step-up sheet, toasts) must show above native modal screens
 * (Mover, Desarrollador). On iOS a root view sits UNDER a presented screen and a root Modal
 * cannot present over it, so these render in react-native-screens' FullWindowOverlay, a
 * window above everything. Android and web already draw them on top.
 */
export function WindowLayer({ children }: { children: ReactNode }) {
  if (!rootSheetUsesWindowOverlay(Platform.OS)) return <Fragment>{children}</Fragment>;
  return <FullWindowOverlay>{children}</FullWindowOverlay>;
}

/** How long the fade-out runs before the overlay window goes away. */
const EXIT_MS = 220;

/**
 * The iOS host of a root sheet: mounted in the overlay window while visible (and during the
 * fade-out). `onShown` fires once it is laid out in that window, i.e. really on screen;
 * `onDismissed` once it is gone, like the Modal events it replaces.
 */
export function WindowSheet({ visible, onShown, onDismissed, children }: { visible: boolean; onShown?: () => void; onDismissed?: () => void; children: ReactNode }) {
  const [mounted, setMounted] = useState(visible);
  const laidOut = useRef(false);
  if (visible && !mounted) setMounted(true);

  // Reopened during the fade-out: the view is already laid out, so onLayout will not fire again.
  useEffect(() => {
    if (visible && laidOut.current) onShown?.();
  }, [visible, onShown]);

  useEffect(() => {
    if (visible || !mounted) return undefined;
    const timer = setTimeout(() => {
      laidOut.current = false;
      setMounted(false);
      onDismissed?.();
    }, EXIT_MS);
    return () => clearTimeout(timer);
  }, [visible, mounted, onDismissed]);

  if (!mounted) return null;
  return (
    <FullWindowOverlay>
      <View
        style={StyleSheet.absoluteFill}
        pointerEvents={visible ? 'auto' : 'none'}
        onLayout={() => {
          laidOut.current = true;
          if (visible) onShown?.();
        }}
      >
        {visible ? (
          <Animated.View entering={FadeIn.duration(EXIT_MS)} exiting={FadeOut.duration(EXIT_MS)} style={StyleSheet.absoluteFill}>
            {children}
          </Animated.View>
        ) : null}
      </View>
    </FullWindowOverlay>
  );
}
