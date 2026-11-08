import { router } from 'expo-router';

import { policySummary } from '@/shared/helpers/approvalPolicy';
import { verifiedPersonalOrgId } from '@/shared/helpers/orgs';
import { canManageTeam, canSeeReconciliation } from '@/shared/helpers/permissions';
import { formatMoney } from '@/shared/money/format';
import { useErpLink } from '@/shared/queries/erp';
import { useActiveOrg, useApprovalPolicy } from '@/shared/queries/orgs';
import { useMe } from '@/shared/queries/session';
import type { IconName } from '@/shared/ui/Icon';

type Href =
  | '/orgs'
  | '/approvals'
  | '/team'
  | '/settings/approval-policies'
  | '/account-permissions'
  | '/reconciliation'
  | '/erp-link'
  | '/onboarding/status'
  | '/my-payments';

export interface MoreItem {
  key: string;
  icon: IconName;
  title: string;
  /** An i18n key, or a ready text (org name), or nothing. */
  subtitle: { key: string; params?: Record<string, string> } | { text: string } | null;
  href: Href;
}

/**
 * "Más" (owner feedback): products, services and the organization's management. The person's own
 * preferences live in Configuración (avatar menu), not here.
 */
export function useMore() {
  const me = useMe();
  const { org } = useActiveOrg();
  const company = org?.kind === 'company';
  const erpLink = useErpLink(company && org.kycStatus === 'approved' ? org.id : undefined);
  const policy = useApprovalPolicy(company ? org.id : undefined);
  const role = org?.role ?? 'viewer';
  const summary = policy.data ? policySummary(policy.data) : null;

  const orgItems: (MoreItem & { show: boolean })[] = [
    { key: 'approvals', icon: 'checkmark-done-outline', title: 'settings.approvals', subtitle: null, href: '/approvals', show: true },
    {
      key: 'policy',
      icon: 'options-outline',
      title: 'policy.title',
      subtitle: summary ? (summary.kind === 'always' ? { key: 'policy.summaryAlways' } : { key: 'policy.summaryThreshold', params: { amounts: summary.thresholds.map((item) => formatMoney(item.minor, item.currency)).join(' · ') } }) : null,
      href: '/settings/approval-policies',
      show: company,
    },
    { key: 'team', icon: 'people-outline', title: 'settings.team', subtitle: canManageTeam(role) ? null : { key: 'settings.readOnly' }, href: '/team', show: true },
    { key: 'permissions', icon: 'key-outline', title: 'settings.accountPermissions', subtitle: { key: 'proposal.ribbon' }, href: '/account-permissions', show: company },
    { key: 'recon', icon: 'git-compare-outline', title: 'settings.reconciliation', subtitle: null, href: '/reconciliation', show: canSeeReconciliation(role) },
    { key: 'erp', icon: 'git-merge-outline', title: 'settings.erp', subtitle: erpLink.data?.status === 'linked' ? { key: 'settings.erpLinked' } : null, href: '/erp-link', show: true },
    { key: 'kyc', icon: 'shield-checkmark-outline', title: 'settings.kyc', subtitle: org ? { key: `status.kyc.${org.kycStatus}` } : null, href: '/onboarding/status', show: true },
    { key: 'switch', icon: 'swap-horizontal', title: 'settings.switchOrg', subtitle: org ? { text: org.legalName } : null, href: '/orgs', show: true },
  ];

  return {
    org,
    // "Mis pagos" belongs to the person, not to the active org: it shows with any org selected.
    hasMyPayments: verifiedPersonalOrgId(me.data?.memberships) !== null,
    orgItems: orgItems.filter((item) => item.show),
    open: (href: Href) => router.push(href),
  };
}
