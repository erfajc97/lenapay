import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { ACCOUNTS_KEY } from '@/shared/queries/accounts';
import { useErpCandidate, useErpLink } from '@/shared/queries/erp';
import { useActiveOrg } from '@/shared/queries/orgs';
import { PAYERS_KEY } from '@/shared/queries/payers';

import { erpStage } from '../helpers/erpRules';
import { useLinkErp } from '../mutations/useLinkErp';

export function useErpLinkScreen() {
  const queryClient = useQueryClient();
  const { org } = useActiveOrg();
  const link = useErpLink(org?.id);
  const stage = erpStage({ kycApproved: org?.kycStatus === 'approved', isAdmin: org?.role === 'admin', linkStatus: link.data?.status });
  const candidate = useErpCandidate(stage === 'candidate' ? org?.id : undefined);
  const [sheetOpen, setSheetOpen] = useState(false);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const linkErp = useLinkErp(org?.id, () => setSheetOpen(false));

  return {
    org,
    stage,
    link,
    candidate,
    sheetOpen,
    openSheet: () => setSheetOpen(true),
    closeSheet: () => setSheetOpen(false),
    username,
    setUsername,
    password,
    setPassword,
    canSubmit: username.trim() !== '' && password !== '',
    submit: () => linkErp.mutate({ username: username.trim(), password }),
    linking: linkErp.isPending,
    // Imported projects and payers are new server data in other screens.
    refreshImported: () => {
      void queryClient.invalidateQueries({ queryKey: ACCOUNTS_KEY(org?.id ?? '') });
      void queryClient.invalidateQueries({ queryKey: PAYERS_KEY(org?.id ?? '') });
    },
  };
}
