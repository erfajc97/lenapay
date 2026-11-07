import { Image as ExpoImage } from 'expo-image';
import { useTranslation } from 'react-i18next';
import { View } from 'react-native';
import { withUniwind } from 'uniwind';

import { AppText } from './AppText';

const Image = withUniwind(ExpoImage);

// Metro picks the @2x/@3x file for the screen density; the sources are cut from assets/brand.
const isotipo = require('../../../assets/logo/lebane-isotipo.png');
const lebaneOnLight = require('../../../assets/logo/lebane-wordmark-on-light.png');
const lebaneOnDark = require('../../../assets/logo/lebane-wordmark-on-dark.png');

const SIZES = {
  large: { mark: 'h-11 w-11', text: 'balance' },
  small: { mark: 'h-8 w-8', text: 'heading' },
} as const;

/**
 * The product mark: the Lebane isotype (parent brand) next to "Lenapay" in Plus Jakarta Sans
 * 700, primary colour. `byLebane` adds the parent wordmark below, in its light or dark
 * version by theme (the two images swap through `dark:`, no JS branch).
 */
export function Wordmark({ size = 'large', byLebane = false }: { size?: keyof typeof SIZES; byLebane?: boolean }) {
  const { t } = useTranslation();
  const { mark, text } = SIZES[size];
  return (
    <View className="gap-1" accessible accessibilityRole="header" accessibilityLabel={byLebane ? t('brand.byLebaneLabel') : t('brand.name')}>
      <View className="flex-row items-center gap-2.5">
        <Image source={isotipo} className={mark} contentFit="contain" cachePolicy="memory" />
        <AppText variant={text} tone="brand">
          {t('brand.name')}
        </AppText>
      </View>
      {byLebane ? (
        <View className="flex-row items-center gap-1.5 pl-1">
          <AppText variant="caption" tone="muted">
            {t('brand.by')}
          </AppText>
          <View className="dark:hidden">
            <Image source={lebaneOnLight} className="h-3.5 w-[60px]" contentFit="contain" cachePolicy="memory" />
          </View>
          <View className="hidden dark:flex">
            <Image source={lebaneOnDark} className="h-3.5 w-[60px]" contentFit="contain" cachePolicy="memory" />
          </View>
        </View>
      ) : null}
    </View>
  );
}
