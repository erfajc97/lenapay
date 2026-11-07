import type { ErpLink } from '@/shared/api/types';

/** Overall import progress 0–100 across projects and payers (R-ERP-3). Nothing to import = 100. */
export function importPercent(link: Pick<ErpLink, 'projects' | 'payers'>): number {
  const total = link.projects.total + link.payers.total;
  if (total === 0) return 100;
  return Math.floor(((link.projects.done + link.payers.done) * 100) / total);
}

type ErpStage = 'blocked_kyc' | 'blocked_role' | 'candidate' | 'importing' | 'linked';

/** R-ERP-2: only after KYC approval and only admins link. */
export function erpStage(params: { kycApproved: boolean; isAdmin: boolean; linkStatus: ErpLink['status'] | undefined }): ErpStage {
  if (params.linkStatus === 'linked') return 'linked';
  if (params.linkStatus === 'importing') return 'importing';
  if (!params.kycApproved) return 'blocked_kyc';
  if (!params.isAdmin) return 'blocked_role';
  return 'candidate';
}
