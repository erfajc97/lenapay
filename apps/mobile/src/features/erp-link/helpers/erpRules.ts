import type { ErpLinkStatus } from '@/shared/api/types';

type ErpStage = 'blocked_kyc' | 'blocked_role' | 'candidate' | 'importing' | 'linked' | 'failed';

/** R-ERP-2: only after KYC approval and only admins link; the import state shows to everyone. */
export function erpStage(params: { kycApproved: boolean; isAdmin: boolean; linkStatus: ErpLinkStatus | undefined }): ErpStage {
  if (params.linkStatus === 'linked' || params.linkStatus === 'importing' || params.linkStatus === 'failed') return params.linkStatus;
  if (!params.kycApproved) return 'blocked_kyc';
  if (!params.isAdmin) return 'blocked_role';
  return 'candidate';
}
