import { useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import type { Cvu } from '@/shared/api/types';
import { moneyActionBlocker } from '@/shared/helpers/permissions';
import { useCopy, useShareText } from '@/shared/mutations/useDeviceActions';
import { useActiveOrg } from '@/shared/queries/orgs';
import { usePayer } from '@/shared/queries/payers';

import { aliasError, missingCurrencies } from '../helpers/payerRules';
import { useDisableCvu, useIssueCvu, useUpdateAlias } from '../mutations/usePayerMutations';

type SheetState = { kind: 'qr' | 'alias' | 'disable'; cvu: Cvu } | null;

export function usePayerDetail() {
  const { t } = useTranslation();
  const params = useLocalSearchParams<{ id?: string }>();
  const payerId = typeof params.id === 'string' ? params.id : undefined;
  const { org } = useActiveOrg();
  const payer = usePayer(org?.id, payerId);
  const [sheet, setSheet] = useState<SheetState>(null);
  const [alias, setAlias] = useState('');
  const closeSheet = () => setSheet(null);
  const copy = useCopy();
  const share = useShareText();
  const issue = useIssueCvu(org?.id, payerId);
  const updateAlias = useUpdateAlias(org?.id, closeSheet);
  const disable = useDisableCvu(org?.id, closeSheet);

  const canEdit = org ? moneyActionBlocker(org) === null : false;
  const aliasInvalid = aliasError(alias);

  return {
    payer,
    canEdit,
    missing: payer.data ? missingCurrencies(payer.data) : [],
    sheet,
    closeSheet,
    openQr: (cvu: Cvu) => setSheet({ kind: 'qr', cvu }),
    openAlias: (cvu: Cvu) => {
      setAlias(cvu.alias);
      setSheet({ kind: 'alias', cvu });
    },
    openDisable: (cvu: Cvu) => setSheet({ kind: 'disable', cvu }),
    alias,
    setAlias,
    aliasError: aliasInvalid,
    saveAlias: () => {
      if (sheet && !aliasInvalid) updateAlias.mutate({ cvuId: sheet.cvu.id, alias });
    },
    savingAlias: updateAlias.isPending,
    confirmDisable: () => {
      if (sheet) disable.mutate(sheet.cvu.id);
    },
    disabling: disable.isPending,
    issue: issue.mutate,
    issuing: issue.isPending,
    copy: copy.mutate,
    share: (cvu: Cvu) =>
      share.mutate(
        t('payers.shareMessage', {
          name: payer.data?.name ?? '',
          currency: t(`currency.${cvu.currency}`),
          cvu: cvu.cvu,
          alias: cvu.alias,
        }),
      ),
  };
}
