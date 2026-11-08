import Ionicons from '@expo/vector-icons/Ionicons';
import { BlurView as ExpoBlurView } from 'expo-blur';
import type { BottomTabBarProps } from 'expo-router/build/layouts/Tabs';
import { useEffect, useRef, useState } from 'react';
import { type LayoutChangeEvent, Platform, Pressable, StyleSheet, View } from 'react-native';
import Animated, { ReduceMotion, useAnimatedStyle, useSharedValue, withSpring } from 'react-native-reanimated';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { withUniwind } from 'uniwind';

import { tabBarBlurs } from '@/shared/helpers/platform';

import { AppText } from './AppText';
import { elevation, SURFACE_TONE } from './elevation';
import { type IconName, useThemeColor } from './Icon';
import { edgeSprings, liquidSquash, pillEdges, TAB_BAR, tabBarSpace } from './liquidTab';

const BlurView = withUniwind(ExpoBlurView);

/**
 * Tab glyph: filled and on-primary on the liquid pill when selected, outlined and muted
 * otherwise. It reads its own theme tokens, so the navigator's tint colour is not used.
 */
export function TabIcon({ name, focused }: { name: IconName; focused: boolean }) {
  const color = useThemeColor(focused ? 'on-primary' : 'text-muted');
  const outline = `${String(name)}-outline` as IconName;
  return <Ionicons name={focused ? name : outline} size={22} color={color} />;
}

/**
 * The liquid pill behind the selected icon. Its edges are two shared values: on a change the
 * edge facing the new tab springs fast and the other follows softly, so the pill stretches
 * toward the target and contracts on arrival, thinning while stretched (UI thread; Reduce
 * Motion makes it jump).
 */
function LiquidPill({ index, count, width }: { index: number; count: number; width: number }) {
  const start = pillEdges({ index, count, width });
  const left = useSharedValue(start.left);
  const right = useSharedValue(start.right);
  const previous = useRef({ index, width });

  useEffect(() => {
    const target = pillEdges({ index, count, width });
    const from = previous.current;
    previous.current = { index, width };
    if (from.width !== width) {
      left.set(target.left);
      right.set(target.right);
      return;
    }
    const springs = edgeSprings({ from: from.index, to: index });
    left.set(withSpring(target.left, { ...springs.left, reduceMotion: ReduceMotion.System }));
    right.set(withSpring(target.right, { ...springs.right, reduceMotion: ReduceMotion.System }));
  }, [index, count, width, left, right]);

  const rest = Math.min(TAB_BAR.pillWidth, count > 0 ? width / count : 0);
  const style = useAnimatedStyle(() => {
    const pillWidth = Math.max(0, right.get() - left.get());
    return { left: left.get(), width: pillWidth, transform: [{ scaleY: liquidSquash({ width: pillWidth, rest }) }] };
  });
  return (
    <Animated.View style={[styles.pill, style]} pointerEvents="none">
      <View className={`h-full w-full rounded-full bg-primary ${elevation('glow')}`} />
    </Animated.View>
  );
}

/**
 * The floating tab bar: a blurred capsule inset from the screen edges, a few dp above the home
 * indicator, with the liquid pill inside. Icons and small labels; the selected tab is also
 * announced (`selected`), never only coloured (M7). Tab screens reserve `tabBarSpace` below.
 */
export function FloatingTabBar({ state, descriptors, emitter, navigateToTab }: BottomTabBarProps) {
  const insets = useSafeAreaInsets();
  const [width, setWidth] = useState(0);
  const blur = tabBarBlurs(Platform.OS);
  const onLayout = (event: LayoutChangeEvent) => setWidth(event.nativeEvent.layout.width);

  return (
    <View pointerEvents="box-none" style={[styles.host, { bottom: insets.bottom + TAB_BAR.gap, left: insets.left + TAB_BAR.side, right: insets.right + TAB_BAR.side }]}>
      <View className={`rounded-full ${elevation('bar')}`}>
        <View className="overflow-hidden rounded-full border border-border-subtle">
          {blur ? <BlurView intensity={40} tint="systemChromeMaterial" className="absolute inset-0" /> : null}
          <View className={`absolute inset-0 ${SURFACE_TONE.bar} ${blur ? 'opacity-80' : ''}`} />
          <View role="tablist" className="flex-row" style={styles.row} onLayout={onLayout}>
            {width > 0 ? <LiquidPill index={state.index} count={state.routes.length} width={width} /> : null}
            {state.routes.map((route, index) => {
              const focused = index === state.index;
              const options = descriptors[route.key]?.options;
              const label = options?.title ?? route.name;
              const onPress = () => {
                const event = emitter.emit({ type: 'tabPress', target: route.key, canPreventDefault: true });
                if (!focused && !event.defaultPrevented) navigateToTab(route.key);
              };
              return (
                <Pressable
                  key={route.key}
                  onPress={onPress}
                  onLongPress={() => emitter.emit({ type: 'tabLongPress', target: route.key })}
                  accessibilityRole="tab"
                  accessibilityLabel={label}
                  accessibilityState={{ selected: focused }}
                  className="flex-1 items-center justify-center gap-0.5"
                >
                  <View style={styles.iconSlot}>{options?.tabBarIcon?.({ focused, color: '', size: 22 })}</View>
                  <AppText variant="caption" tone={focused ? 'brand' : 'muted'} numberOfLines={1} className="font-sans-semibold text-[10px]">
                    {label}
                  </AppText>
                </Pressable>
              );
            })}
          </View>
        </View>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  host: { position: 'absolute' },
  row: { height: TAB_BAR.height, paddingTop: 5, paddingBottom: 5 },
  iconSlot: { height: TAB_BAR.pillHeight, alignItems: 'center', justifyContent: 'center' },
  pill: { position: 'absolute', top: 6, height: TAB_BAR.pillHeight },
});

/** Bottom padding of a tab screen's scroll or list so its last row clears the floating bar. */
export function useTabBarSpace(): number {
  return tabBarSpace(useSafeAreaInsets().bottom);
}
