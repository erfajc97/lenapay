import { router } from 'expo-router';
import { useTranslation } from 'react-i18next';

import { Screen } from '@/shared/ui/Screen';
import { EmptyState } from '@/shared/ui/States';

export function NotFound() {
  const { t } = useTranslation();
  return (
    <Screen back>
      <EmptyState icon="compass-outline" title={t('notFound.title')} body={t('notFound.body')} actionLabel={t('notFound.action')} onAction={() => router.replace('/')} />
    </Screen>
  );
}
