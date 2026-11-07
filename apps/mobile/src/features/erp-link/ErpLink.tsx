import { router } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { isMockApi } from '@/shared/api';
import { formatCuit } from '@/shared/helpers/cuit';
import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { Icon } from '@/shared/ui/Icon';
import { Field, ListRow, Notice } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { Sheet } from '@/shared/ui/Sheet';
import { EmptyState, ErrorState, SkeletonList } from '@/shared/ui/States';
import { TextField } from '@/shared/ui/TextField';

import { ProgressBar } from './components/ProgressBar';
import { useErpLinkScreen } from './hooks/useErpLinkScreen';

/** Link with the Lebane ERP by CUIT, confirm with ERP credentials, import (R-ERP-1..3). */
export function ErpLink() {
  const { t } = useTranslation();
  const screen = useErpLinkScreen();
  const link = screen.link.data;
  const candidate = screen.candidate.data;

  return (
    <Screen title={t('erp.title')} back>
      <View className="items-center gap-2 py-2">
        <View className="h-16 w-16 items-center justify-center rounded-full bg-primary-bg">
          <Icon name="git-merge-outline" size={30} tone="primary" />
        </View>
        <AppText tone="secondary" className="text-center">
          {t('erp.intro')}
        </AppText>
      </View>

      {screen.link.isLoading ? <SkeletonList rows={2} /> : null}
      {screen.link.isError ? <ErrorState error={screen.link.error} onRetry={() => void screen.link.refetch()} /> : null}

      {screen.stage === 'blocked_kyc' ? <Notice tone="warning" icon="shield-outline" title={t('blockers.kyc')} /> : null}
      {screen.stage === 'blocked_role' ? <Notice tone="warning" icon="lock-closed-outline" title={t('erp.adminOnly')} /> : null}

      {screen.stage === 'candidate' ? (
        <>
          {screen.candidate.isLoading ? <SkeletonList rows={1} /> : null}
          {screen.candidate.isError ? <ErrorState error={screen.candidate.error} onRetry={() => void screen.candidate.refetch()} /> : null}
          {candidate && !candidate.found ? <EmptyState icon="search-outline" title={t('erp.notFoundTitle')} body={t('erp.notFoundBody')} /> : null}
          {candidate?.found ? (
            <Card className="gap-2">
              <AppText variant="label" tone="brand">
                {t('erp.found')}
              </AppText>
              <AppText variant="subheading">{candidate.erpOrgName ?? ''}</AppText>
              {candidate.cuit ? <Field label={t('fields.cuit')} value={formatCuit(candidate.cuit)} mono /> : null}
              <View className="flex-row gap-3">
                <Card className="flex-1 items-center gap-1">
                  <AppText variant="heading">{String(candidate.projectsCount)}</AppText>
                  <AppText variant="caption" tone="muted">
                    {t('erp.projects')}
                  </AppText>
                </Card>
                <Card className="flex-1 items-center gap-1">
                  <AppText variant="heading">{String(candidate.buyersCount)}</AppText>
                  <AppText variant="caption" tone="muted">
                    {t('erp.buyers')}
                  </AppText>
                </Card>
              </View>
              <Button label={t('erp.link')} icon="link-outline" onPress={screen.openSheet} />
            </Card>
          ) : null}
        </>
      ) : null}

      {screen.stage === 'importing' && link ? (
        <Card className="gap-4">
          <AppText variant="subheading">{t('erp.importing', { percent: screen.percent })}</AppText>
          <ProgressBar label={t('erp.projects')} done={link.projects.done} total={link.projects.total} />
          <ProgressBar label={t('erp.buyers')} done={link.payers.done} total={link.payers.total} />
        </Card>
      ) : null}

      {screen.stage === 'linked' && link ? (
        <>
          <Notice tone="success" icon="checkmark-circle-outline" title={t('erp.linkedTitle')} body={t('erp.linkedBody', { count: link.payers.total })} />
          <Card className="py-1">
            {link.importedProjects.map((project) => (
              <ListRow key={project} title={project} subtitle={t('erp.importedProject')} />
            ))}
          </Card>
          <Button
            label={t('erp.seeAccounts')}
            variant="secondary"
            onPress={() => {
              screen.refreshImported();
              router.replace('/accounts');
            }}
          />
        </>
      ) : null}

      <Sheet visible={screen.sheetOpen} title={t('erp.credentialsTitle')} onClose={screen.closeSheet}>
        <AppText tone="secondary">{t('erp.credentialsBody')}</AppText>
        {isMockApi ? <Notice tone="info" icon="flask-outline" title={t('demo.title')} body={t('demo.erp')} /> : null}
        <TextField label={t('fields.lebaneUser')} value={screen.username} onChangeText={screen.setUsername} keyboardType="email-address" autoCapitalize="none" autoComplete="username" textContentType="username" returnKeyType="next" />
        <TextField label={t('fields.password')} value={screen.password} onChangeText={screen.setPassword} secureTextEntry autoComplete="current-password" textContentType="password" returnKeyType="done" onSubmitEditing={screen.submit} />
        <Button label={t('erp.confirmLink')} onPress={screen.submit} loading={screen.linking} disabled={!screen.canSubmit} />
      </Sheet>
    </Screen>
  );
}
