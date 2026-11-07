import { useTranslation } from 'react-i18next';

import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { LinkButton, Notice } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { TextField } from '@/shared/ui/TextField';

import { KindCard } from './components/KindCard';
import { useOnboarding } from './hooks/useOnboarding';

/** Onboarding start: who is opening the account (R-KYC-1, R-KYC-2). */
export function Onboarding() {
  const { t } = useTranslation();
  const screen = useOnboarding();
  return (
    <Screen title={t('onboarding.title')} back footer={<Button label={t('common.continue')} onPress={screen.submit} loading={screen.creating} />}>
      {screen.pendingOrg ? (
        <Notice
          tone="info"
          icon="document-text-outline"
          title={t('onboarding.pendingTitle', { name: screen.pendingOrg.legalName })}
          action={<LinkButton label={t('onboarding.pendingAction')} onPress={screen.continuePending} />}
        />
      ) : null}
      <AppText tone="secondary">{t('onboarding.body')}</AppText>
      <KindCard icon="business-outline" title={t('onboarding.company')} body={t('onboarding.companyBody')} selected={screen.kind === 'company'} onPress={() => screen.setKind('company')} />
      <KindCard
        icon="person-outline"
        title={t('onboarding.person')}
        body={screen.personalEnabled ? t('onboarding.personBody') : t('onboarding.personLocked')}
        selected={screen.kind === 'personal'}
        disabled={!screen.personalEnabled}
        onPress={() => screen.setKind('personal')}
      />
      <TextField
        label={screen.kind === 'company' ? t('fields.legalName') : t('fields.fullName')}
        value={screen.legalName}
        onChangeText={screen.setLegalName}
        error={screen.error ? t(screen.error) : null}
        autoCapitalize="words"
        autoComplete={screen.kind === 'company' ? 'organization' : 'name'}
        textContentType={screen.kind === 'company' ? 'organizationName' : 'name'}
        returnKeyType="done"
        onSubmitEditing={screen.submit}
      />
    </Screen>
  );
}
