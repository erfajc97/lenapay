import { useTranslation } from 'react-i18next';

import { useIsMockApi } from '@/shared/api';

import { Chip } from './Chip';

/** "Modo demo" while the app runs on the mock backend; nothing at all against the real API. */
export function DemoBadge() {
  const { t } = useTranslation();
  const isMock = useIsMockApi();
  return isMock ? <Chip label={t('common.demoMode')} tone="warning" /> : null;
}
