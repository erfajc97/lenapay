import { Redirect, router } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { cvuQrPayload } from '@/shared/helpers/cbu';
import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { CvuQr } from '@/shared/ui/CvuQr';
import { DemoBadge } from '@/shared/ui/DemoBadge';
import { MovementRow } from '@/shared/ui/MovementRow';
import { Divider, LinkButton, Notice, SectionTitle } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { Sheet } from '@/shared/ui/Sheet';
import { EmptyState, ErrorState, SkeletonList } from '@/shared/ui/States';
import { TextField } from '@/shared/ui/TextField';

import { BalanceCard } from './components/BalanceCard';
import { MyCvuCard } from './components/MyCvuCard';
import { MyPaymentsEntry } from './components/MyPaymentsEntry';
import { AccountMenu } from './components/AccountMenu';
import { OrgHeader } from './components/OrgHeader';
import { QuickActions } from './components/QuickActions';
import { useHome } from './hooks/useHome';

type HomeScreen = ReturnType<typeof useHome>;

/** "Mi CVU" sheets: the QR to collect and the alias editor. */
function PersonalSheets({ home }: { home: HomeScreen }) {
  const { t } = useTranslation();
  const { wallet } = home;
  const { sheet } = wallet;
  return (
    <>
      <Sheet visible={sheet?.kind === 'qr'} title={t('home.myCvu.qrTitle')} onClose={wallet.closeSheet}>
        {sheet ? (
          <View className="items-center gap-3">
            <CvuQr value={cvuQrPayload(sheet.cvu)} label={t('home.myCvu.qrA11y', { currency: t(`currency.${sheet.cvu.currency}`) })} />
            <AppText variant="bodyStrong">{home.org?.legalName ?? ''}</AppText>
            {sheet.cvu.alias ? (
              <AppText variant="mono" selectable>
                {sheet.cvu.alias}
              </AppText>
            ) : null}
            <AppText variant="monoSmall" tone="muted" selectable>
              {sheet.cvu.cvu}
            </AppText>
            <Button label={t('common.share')} icon="share-outline" onPress={() => wallet.share(sheet.cvu)} />
          </View>
        ) : null}
      </Sheet>
      <Sheet visible={sheet?.kind === 'alias'} title={t('payers.editAlias')} onClose={wallet.closeSheet}>
        <TextField
          label={t('fields.alias')}
          value={wallet.alias}
          onChangeText={wallet.setAlias}
          error={wallet.aliasError ? t(wallet.aliasError) : null}
          hint={t('payers.aliasHint')}
          autoCapitalize="none"
          autoCorrect={false}
          autoComplete="off"
          mono
          returnKeyType="done"
          onSubmitEditing={wallet.saveAlias}
          autoFocus
        />
        <Button label={t('common.save')} onPress={wallet.saveAlias} loading={wallet.savingAlias} disabled={wallet.aliasError !== null} />
      </Sheet>
    </>
  );
}

export function Home() {
  const { t } = useTranslation();
  const home = useHome();

  if (home.noOrgs) return <Redirect href="/onboarding" />;
  if (home.orgsQuery.isError) {
    return (
      <Screen inTabs>
        <ErrorState error={home.orgsQuery.error} onRetry={home.refresh} />
      </Screen>
    );
  }
  if (!home.org || !home.actions) {
    return (
      <Screen inTabs>
        <SkeletonList rows={4} />
      </Screen>
    );
  }

  const { org } = home;
  return (
    <Screen inTabs onRefresh={home.refresh}>
      <OrgHeader org={org} firstName={home.firstName} menuOpen={home.menu.open} onSwitch={() => router.push('/orgs')} onMenu={home.menu.show} />
      <AccountMenu
        visible={home.menu.open}
        name={home.myName}
        email={home.myEmail}
        orgName={org.legalName}
        confirmLogout={home.menu.confirmLogout}
        onPick={home.menu.pick}
        onLogout={home.menu.logout}
        onClose={home.menu.close}
        onDismissed={home.menu.dismissed}
      />
      <DemoBadge />
      {home.kycNotice ? (
        <Notice
          tone={home.kycNotice === 'rejected' ? 'danger' : 'warning'}
          icon="shield-outline"
          title={t(`${home.kycCopy}.${home.kycNotice}.title`)}
          body={t(`${home.kycCopy}.${home.kycNotice}.body`)}
          action={<LinkButton label={t(`${home.kycCopy}.${home.kycNotice}.action`)} onPress={home.openKyc} />}
        />
      ) : null}
      {home.balances.isError ? (
        <ErrorState error={home.balances.error} onRetry={() => void home.balances.refetch()} />
      ) : (
        <BalanceCard currency={home.currency} onCurrency={home.setCurrency} balance={home.balance} loading={home.balances.isLoading} onOpenHeld={home.openHeld} onOpenTotal={home.openTotal} />
      )}
      <QuickActions
        state={home.actions}
        onCollect={home.collect}
        onPay={() => router.push('/pay')}
        onMove={() => router.push('/move')}
        onBlocked={home.explainBlocked}
      />
      {home.personal && home.wallet.cvus.length > 0 ? (
        <MyCvuCard cvus={home.wallet.cvus} onCopy={home.wallet.copy} onShare={home.wallet.share} onQr={home.wallet.openQr} onEditAlias={home.wallet.openAlias} />
      ) : null}
      {home.wallet.showMyPayments ? <MyPaymentsEntry onOpen={() => router.push('/my-payments')} /> : null}
      {home.pendingApprovals > 0 ? (
        <Notice
          tone="info"
          icon="checkmark-done-outline"
          title={t('home.approvalsTitle', { count: home.pendingApprovals })}
          body={t('home.approvalsBody')}
          action={<LinkButton label={t('home.approvalsAction')} onPress={() => router.push('/approvals')} />}
        />
      ) : null}
      {/* Title and list read as one block: tighter than the screen's gap-4 rhythm. */}
      <View className="gap-2">
        <SectionTitle title={t('home.recent')} action={home.recent.length > 0 ? <LinkButton label={t('common.seeAll')} onPress={() => router.push('/movements')} /> : undefined} />
        <Card className="py-1">
          {home.movements.isLoading ? <SkeletonList rows={3} /> : null}
          {home.movements.isError ? <ErrorState error={home.movements.error} onRetry={() => void home.movements.refetch()} /> : null}
          {(home.movements.isSuccess || org.kycStatus !== 'approved') && home.recent.length === 0 ? (
            <EmptyState icon="receipt-outline" title={t('home.emptyTitle')} body={t('home.emptyBody')} actionLabel={t('home.collect')} onAction={() => router.push('/payers')} />
          ) : null}
          {home.recent.map((movement, index) => (
            <View key={movement.id}>
              {index > 0 ? <Divider /> : null}
              <MovementRow movement={movement} onPress={home.openMovement} />
            </View>
          ))}
        </Card>
      </View>
      {home.erpInvite ? (
        <Card className="gap-3">
          <SectionTitle title={t('home.erpTitle')} />
          <Button label={t('home.erpAction')} variant="secondary" icon="link-outline" onPress={() => router.push('/erp-link')} compact />
        </Card>
      ) : null}
      <PersonalSheets home={home} />
    </Screen>
  );
}
