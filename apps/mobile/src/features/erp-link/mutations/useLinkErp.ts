import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import type { ErpLinkRequest } from '@/shared/api/types';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { ERP_LINK_KEY } from '@/shared/queries/erp';
import { ORGS_KEY } from '@/shared/queries/orgs';

/** Confirm the link with the ERP credentials (R-ERP-2); the import then runs on the API. */
export function useLinkErp(orgId: string | undefined, onDone: () => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: ErpLinkRequest) => api.erp.link(orgId ?? '', body),
    onSuccess: (link) => {
      queryClient.setQueryData(ERP_LINK_KEY(orgId ?? ''), link);
      void queryClient.invalidateQueries({ queryKey: ORGS_KEY });
      notifySuccess('erp.linkStarted');
      onDone();
    },
    onError: notifyError,
  });
}
