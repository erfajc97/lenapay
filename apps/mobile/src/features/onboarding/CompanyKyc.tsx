import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { formatDni } from '@/shared/helpers/cuit';
import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { FormField } from '@/shared/ui/FormField';
import { Divider, Notice, SectionTitle } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { Sheet } from '@/shared/ui/Sheet';
import { ErrorState, SkeletonList } from '@/shared/ui/States';

import { Checklist } from './components/Checklist';
import { DocumentRow } from './components/DocumentRow';
import { MockCameraSheet } from './components/MockCameraSheet';
import { hasCompanyDocument, representativeDocs } from './helpers/kycChecklist';
import { useCompanyKyc } from './hooks/useKycForms';

/** Company KYC: data, documents, representatives with DNI + selfie (R-KYC-1). */
export function CompanyKyc() {
  const { t } = useTranslation();
  const kyc = useCompanyKyc();
  const data = kyc.kyc.data;

  if (kyc.kyc.isError) {
    return (
      <Screen back>
        <ErrorState error={kyc.kyc.error} onRetry={() => void kyc.kyc.refetch()} />
      </Screen>
    );
  }
  if (!data) {
    return (
      <Screen title={t('kyc.companyTitle')} back>
        <SkeletonList rows={4} />
      </Screen>
    );
  }

  return (
    <Screen title={t('kyc.companyTitle')} back footer={<Button label={t('kyc.submit')} icon="send" onPress={kyc.submit} loading={kyc.submitting} disabled={!kyc.canSubmit} />}>
      {data.status === 'rejected' && data.rejectionReason ? <Notice tone="danger" icon="close-circle-outline" title={t('kyc.rejectedTitle')} body={data.rejectionReason} /> : null}
      <Checklist items={kyc.checklist} />

      <SectionTitle title={t('kyc.companyData')} />
      <Card className="gap-3">
        <FormField control={kyc.form.control} name="cuit" label={t('fields.cuit')} keyboardType="number-pad" autoComplete="off" mono returnKeyType="next" />
        <FormField control={kyc.form.control} name="legalName" label={t('fields.legalName')} autoCapitalize="words" autoComplete="organization" textContentType="organizationName" returnKeyType="next" />
        <FormField control={kyc.form.control} name="legalAddress" label={t('fields.legalAddress')} autoComplete="street-address" textContentType="fullStreetAddress" returnKeyType="done" />
        <Button label={t('common.save')} variant="secondary" onPress={kyc.saveCompany} loading={kyc.saving} compact />
      </Card>

      <SectionTitle title={t('kyc.documents')} />
      <Card className="py-1">
        <DocumentRow label={t('kyc.doc.bylaws')} done={hasCompanyDocument(data, 'bylaws')} actionLabel={t('kyc.upload')} onAction={() => kyc.upload('bylaws')} busy={kyc.uploading} />
        <Divider />
        <DocumentRow label={t('kyc.doc.afip_constancy')} done={hasCompanyDocument(data, 'afip_constancy')} actionLabel={t('kyc.upload')} onAction={() => kyc.upload('afip_constancy')} busy={kyc.uploading} />
      </Card>

      <SectionTitle title={t('kyc.representatives')} />
      {data.representatives.map((rep) => {
        const docs = representativeDocs(data, rep.id);
        return (
          <Card key={rep.id} className="gap-1">
            <AppText variant="bodyStrong">{rep.fullName}</AppText>
            <AppText variant="monoSmall" tone="muted">
              {t('kyc.dniLine', { dni: formatDni(rep.dni) })}
            </AppText>
            <DocumentRow label={t('kyc.doc.dni_front')} done={docs.dniFront} actionLabel={t('kyc.upload')} onAction={() => kyc.upload('dni_front', rep.id)} busy={kyc.uploading} />
            <DocumentRow label={t('kyc.doc.selfie')} done={docs.selfie} actionLabel={t('kyc.takeSelfie')} onAction={() => kyc.openCamera(rep.id)} busy={kyc.uploading} />
          </Card>
        );
      })}
      <View>
        <Button label={t('kyc.addRepresentative')} variant="secondary" icon="person-add-outline" onPress={kyc.openRepSheet} compact />
      </View>

      <Sheet visible={kyc.repSheet} title={t('kyc.addRepresentative')} onClose={kyc.closeRepSheet}>
        <FormField control={kyc.repForm.control} name="fullName" label={t('fields.fullName')} autoCapitalize="words" autoComplete="name" textContentType="name" returnKeyType="next" />
        <FormField control={kyc.repForm.control} name="dni" label={t('fields.dni')} keyboardType="number-pad" autoComplete="off" mono returnKeyType="done" />
        <Button label={t('kyc.addRepresentative')} onPress={kyc.addRepresentative} loading={kyc.saving} />
      </Sheet>
      <MockCameraSheet visible={kyc.camera !== null} onClose={kyc.closeCamera} onCapture={kyc.capture} busy={kyc.uploading} />
    </Screen>
  );
}
