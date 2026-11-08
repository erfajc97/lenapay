import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { Chip, type ChipTone } from '@/shared/ui/Chip';
import { Field, Notice } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { Segmented } from '@/shared/ui/Segmented';
import { TextField } from '@/shared/ui/TextField';

import { ConnectionResult } from './components/ConnectionResult';
import type { ConnectionChipState } from './helpers/connectionCheck';
import { useDeveloper } from './hooks/useDeveloper';

const CHIP_TONE: Record<ConnectionChipState, ChipTone> = { untested: 'neutral', checking: 'brand', connected: 'success', offline: 'danger' };

/** Settings → Desarrollador: switch mock / live and the API URL on the device (dev builds). */
export function Developer() {
  const { t } = useTranslation();
  const dev = useDeveloper();

  if (!dev.enabled) {
    return (
      <Screen title={t('developer.title')} back>
        <Notice tone="info" icon="information-circle-outline" title={t('developer.unavailable')} />
      </Screen>
    );
  }

  return (
    <Screen
      title={t('developer.title')}
      back
      footer={
        <View className="gap-3">
          <Button label={t('developer.apply')} onPress={dev.apply} loading={dev.applying} disabled={!dev.canApply} />
          <Button label={t('developer.reset')} variant="ghost" onPress={dev.reset} disabled={dev.applying} />
        </View>
      }
    >
      <View className="flex-row items-center gap-2" accessible accessibilityLabel={t('developer.connection.chipA11y', { state: t(`developer.connection.chip.${dev.chip}`) })}>
        <AppText variant="caption" tone="muted">
          {t('developer.connection.chipLabel')}
        </AppText>
        <Chip label={t(`developer.connection.chip.${dev.chip}`)} tone={CHIP_TONE[dev.chip]} />
      </View>
      <Card className="gap-1">
        <Field label={t('developer.current')} value={t(`developer.mode.${dev.current.mode}`)} />
        {dev.current.mode === 'live' ? <Field label={t('developer.url')} value={dev.current.url} mono /> : null}
        <Field label={t('developer.buildDefault')} value={`${t(`developer.mode.${dev.buildDefault.mode}`)} · ${dev.buildDefault.url}`} />
      </Card>
      <Segmented
        label={t('developer.modeLabel')}
        value={dev.mode}
        onChange={dev.setMode}
        options={[
          { value: 'mock', label: t('developer.mode.mock') },
          { value: 'live', label: t('developer.mode.live') },
        ]}
      />
      {dev.mode === 'live' ? (
        <View className="gap-3">
          <TextField
            label={t('developer.url')}
            value={dev.url}
            onChangeText={dev.setUrl}
            error={dev.urlError ? t(dev.urlError) : null}
            hint={t('developer.urlHint')}
            keyboardType="url"
            autoCapitalize="none"
            autoCorrect={false}
            autoComplete="off"
            textContentType="URL"
            returnKeyType="done"
            mono
          />
          <Button label={t('developer.check')} variant="secondary" icon="pulse-outline" onPress={dev.check} loading={dev.checking} disabled={!dev.canCheck} compact />
          {dev.checking ? <Notice tone="info" icon="pulse-outline" title={t('developer.connection.checkingTitle')} body={t('developer.connection.checkingBody', { seconds: dev.checkTimeoutSeconds })} /> : null}
          {dev.result ? <ConnectionResult result={dev.result} onCopyCommand={dev.copyFirewallCommand} copied={dev.firewallCommandCopied} /> : null}
        </View>
      ) : null}
      <AppText variant="caption" tone="muted">
        {t('developer.signOutNote')}
      </AppText>
    </Screen>
  );
}
