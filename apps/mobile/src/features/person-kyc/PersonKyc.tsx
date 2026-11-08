import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { AddressFields } from '@/shared/kyc/AddressFields';
import { Checklist } from '@/shared/kyc/Checklist';
import { DocumentRow } from '@/shared/kyc/DocumentRow';
import { IDENTITY_DOCS, identityDocs } from '@/shared/kyc/kycChecklist';
import { MockCameraSheet } from '@/shared/kyc/MockCameraSheet';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { FormField } from '@/shared/ui/FormField';
import { Divider, Notice, SectionTitle } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { ErrorState, SkeletonList } from '@/shared/ui/States';

import { usePersonKyc } from './hooks/usePersonKyc';

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
      {data.status === 'rejected' && data.decisionReason ? <Notice tone="danger" icon="close-circle-outline" title={t('kyc.rejectedTitle')} body={data.decisionReason} /> : null}
      <Checklist items={kyc.checklist} />
      <SectionTitle title={t('kyc.personData')} />
      <Card className="gap-3">
        <FormField control={kyc.form.control} name="fullName" label={t('fields.fullName')} autoCapitalize="words" autoComplete="name" textContentType="name" returnKeyType="next" />
        <FormField control={kyc.form.control} name="dni" label={t('fields.dni')} keyboardType="number-pad" autoComplete="off" mono returnKeyType="next" />
        <FormField
          control={kyc.form.control}
          name="birthDate"
          label={t('fields.birthDate')}
          placeholder={t('movements.filter.dayPlaceholder')}
          keyboardType="numbers-and-punctuation"
          autoComplete="birthdate-full"
          textContentType="birthdate"
          returnKeyType="next"
        />
        <AddressFields control={kyc.form.control} />
        <Button label={t('common.save')} variant="secondary" onPress={kyc.savePerson} loading={kyc.saving} compact />
      </Card>
      <SectionTitle title={t('kyc.identity')} />
      <Card className="py-1">
        {IDENTITY_DOCS.map((kind, index) => (
          <View key={kind}>
            {index > 0 ? <Divider /> : null}
            <DocumentRow
              label={t(`kyc.doc.${kind}`)}
              done={identityDocs(data, null)[kind]}
              actionLabel={kind === 'selfie' ? t('kyc.takeSelfie') : t('kyc.upload')}
              onAction={() => (kind === 'selfie' ? kyc.takeSelfie() : kyc.upload(kind))}
              busy={kyc.uploading}
            />
          </View>
        ))}
      </Card>
      <MockCameraSheet visible={kyc.camera !== null} onClose={kyc.closeCamera} onCapture={kyc.capture} busy={kyc.uploading} />
    </Screen>
  );
}
