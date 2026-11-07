import { useTranslation } from 'react-i18next';

import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { FormField } from '@/shared/ui/FormField';
import { Divider, Notice, SectionTitle } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { ErrorState, SkeletonList } from '@/shared/ui/States';

import { Checklist } from './components/Checklist';
import { DocumentRow } from './components/DocumentRow';
import { MockCameraSheet } from './components/MockCameraSheet';
import { hasPersonDocument } from './helpers/kycChecklist';
import { usePersonKyc } from './hooks/useKycForms';

/** Person KYC: DNI + selfie (R-KYC-2). */
export function PersonKyc() {
  const { t } = useTranslation();
  const kyc = usePersonKyc();
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
      <Screen title={t('kyc.personTitle')} back>
        <SkeletonList rows={3} />
      </Screen>
    );
  }

  return (
    <Screen title={t('kyc.personTitle')} back footer={<Button label={t('kyc.submit')} icon="send" onPress={kyc.submit} loading={kyc.submitting} disabled={!kyc.canSubmit} />}>
      {data.status === 'rejected' && data.rejectionReason ? <Notice tone="danger" icon="close-circle-outline" title={t('kyc.rejectedTitle')} body={data.rejectionReason} /> : null}
      <Checklist items={kyc.checklist} />
      <SectionTitle title={t('kyc.personData')} />
      <Card className="gap-3">
        <FormField control={kyc.form.control} name="fullName" label={t('fields.fullName')} autoCapitalize="words" autoComplete="name" textContentType="name" returnKeyType="next" />
        <FormField control={kyc.form.control} name="dni" label={t('fields.dni')} keyboardType="number-pad" autoComplete="off" mono returnKeyType="done" />
        <Button label={t('common.save')} variant="secondary" onPress={kyc.savePerson} loading={kyc.saving} compact />
      </Card>
      <SectionTitle title={t('kyc.identity')} />
      <Card className="py-1">
        <DocumentRow label={t('kyc.doc.dni_front')} done={hasPersonDocument(data, 'dni_front')} actionLabel={t('kyc.upload')} onAction={() => kyc.upload('dni_front')} busy={kyc.uploading} />
        <Divider />
        <DocumentRow label={t('kyc.doc.selfie')} done={hasPersonDocument(data, 'selfie')} actionLabel={t('kyc.takeSelfie')} onAction={() => kyc.openCamera()} busy={kyc.uploading} />
      </Card>
      <MockCameraSheet visible={kyc.camera !== null} onClose={kyc.closeCamera} onCapture={kyc.capture} busy={kyc.uploading} />
    </Screen>
  );
}
