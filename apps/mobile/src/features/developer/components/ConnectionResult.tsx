import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Notice } from '@/shared/ui/Rows';

import type { ConnectionCheckView } from '../helpers/connectionCheck';

/** The inline card under "Probar conexión": green with the latency, or red with the cause and next step. */
export function ConnectionResult({ result, onCopyCommand, copied }: { result: ConnectionCheckView; onCopyCommand: () => void; copied: boolean }) {
  const { t } = useTranslation();
  const success = result.tone === 'success';
  return (
    <View accessibilityLiveRegion="polite">
      <Notice
        tone={result.tone}
        icon={success ? 'checkmark-circle-outline' : 'alert-circle-outline'}
        title={t(result.titleKey, result.params)}
        body={t(result.bodyKey, result.params)}
        action={
          result.firewallCommand ? (
            <View className="mt-1 gap-2">
              <AppText variant="caption" tone="secondary">
                {t('developer.connection.firewallHint')}
              </AppText>
              <View className="rounded-input bg-bg-surface p-2">
                <AppText variant="monoSmall" selectable>
                  {result.firewallCommand}
                </AppText>
              </View>
              <Button
                label={copied ? t('developer.connection.copied') : t('developer.connection.copyCommand')}
                variant="secondary"
                icon={copied ? 'checkmark' : 'copy-outline'}
                onPress={onCopyCommand}
                compact
              />
            </View>
          ) : null
        }
      />
    </View>
  );
}
